package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type generatedReader struct {
	remaining int64
	offset    int64
}

func (reader *generatedReader) Read(buffer []byte) (int, error) {
	if reader.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(buffer)) > reader.remaining {
		buffer = buffer[:reader.remaining]
	}
	for index := range buffer {
		buffer[index] = byte(((reader.offset+int64(index))*31 + 17) % 251)
	}
	reader.offset += int64(len(buffer))
	reader.remaining -= int64(len(buffer))
	return len(buffer), nil
}

type fileMetadata struct {
	ID       uint64 `json:"id"`
	UID      string `json:"uid"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	FileName string `json:"fileName"`
}

func runFile(arguments []string) error {
	flags := flag.NewFlagSet("file", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "../.tmp/perf/manifest.json", "manifest path")
	size := flags.Int64("size", 1<<20, "generated file size in bytes")
	concurrency := flags.Int("concurrency", 1, "concurrent uploads/downloads")
	dedup := flags.Bool("dedup", false, "use the same filename for every concurrent upload")
	cleanup := flags.Bool("cleanup", true, "delete created UserFile references through the public API")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *size < 0 || *size > 50<<20 || *concurrency < 1 || *concurrency > 50 {
		return fmt.Errorf("invalid file workload parameters")
	}
	value, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	password, err := requiredSecret(passwordEnv)
	if err != nil {
		return err
	}
	api, err := newAPIClient(value.BaseURL, 2*time.Minute)
	if err != nil {
		return err
	}
	loggedIn, err := api.login(context.Background(), value.Users[0].Account, password)
	if err != nil {
		return err
	}
	expectedHash := generatedSHA256(*size)
	runID := strconv.FormatInt(time.Now().UnixNano(), 36)
	started := time.Now()
	var uploadStats recorder
	var uploadBytes atomic.Int64
	metadatas := make([]fileMetadata, *concurrency)
	var workers sync.WaitGroup
	for index := 0; index < *concurrency; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			filename := fmt.Sprintf("perf-%s-%02d.bin", runID, index)
			if *dedup {
				filename = fmt.Sprintf("perf-dedup-%s.bin", runID)
			}
			requestStarted := time.Now()
			metadata, uploadErr := uploadGenerated(context.Background(), api, loggedIn.Token, filename, *size)
			if uploadErr != nil {
				uploadStats.failure(timedOut(uploadErr), false, !timedOut(uploadErr))
				return
			}
			if metadata.Size != *size || metadata.SHA256 != expectedHash {
				uploadStats.failure(false, true, false)
				return
			}
			metadatas[index] = metadata
			uploadBytes.Add(*size)
			uploadStats.success(time.Since(requestStarted))
		}(index)
	}
	workers.Wait()
	uploadElapsed := time.Since(started)
	unique := make(map[uint64]fileMetadata)
	uniqueUID := make(map[string]struct{})
	for _, metadata := range metadatas {
		if metadata.ID != 0 {
			unique[metadata.ID] = metadata
			uniqueUID[metadata.UID] = struct{}{}
		}
	}
	downloadStarted := time.Now()
	var downloadStats recorder
	var downloadBytes atomic.Int64
	for _, metadata := range unique {
		workers.Add(1)
		go func(metadata fileMetadata) {
			defer workers.Done()
			requestStarted := time.Now()
			bytesRead, downloadErr := downloadFile(context.Background(), api, loggedIn.Token, metadata.ID)
			if downloadErr != nil || bytesRead != metadata.Size {
				downloadStats.failure(timedOut(downloadErr), downloadErr == nil, false)
				return
			}
			downloadBytes.Add(bytesRead)
			downloadStats.success(time.Since(requestStarted))
		}(metadata)
	}
	workers.Wait()
	downloadElapsed := time.Since(downloadStarted)
	db, err := openPerfDB()
	if err != nil {
		return err
	}
	defer db.Close()
	var objectCount, referenceCount uint64
	if err := db.QueryRow(`SELECT COUNT(*) FROM file_objects WHERE sha256=?`, expectedHash).Scan(&objectCount); err != nil {
		return fmt.Errorf("verify FileObject count failed")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_files uf JOIN file_objects fo ON fo.id=uf.file_object_id WHERE uf.user_id=? AND fo.sha256=?`, loggedIn.User.ID, expectedHash).Scan(&referenceCount); err != nil {
		return fmt.Errorf("verify UserFile count failed")
	}
	if *cleanup {
		for id := range unique {
			envelope, status, deleteErr := api.request(context.Background(), http.MethodDelete, "/api/file/user-files/"+strconv.FormatUint(id, 10), loggedIn.Token, "", nil)
			if deleteErr != nil || status != http.StatusOK || envelope.Code != 0 {
				return fmt.Errorf("cleanup UserFile reference %d failed", id)
			}
		}
	}
	expectedReferences := uint64(*concurrency)
	if *dedup {
		expectedReferences = 1
	}
	correct := uploadStats.summarize(uploadElapsed).Failures == 0 && downloadStats.summarize(downloadElapsed).Failures == 0 && objectCount == 1 && referenceCount >= expectedReferences && len(uniqueUID) == 1
	return printJSON(map[string]any{
		"workload": "file.stream", "gatewayPath": true, "fileSizeBytes": *size, "concurrency": *concurrency,
		"dedupSameFilename": *dedup, "upload": uploadStats.summarize(uploadElapsed),
		"uploadMiBPerSecond":     float64(uploadBytes.Load()) / (1 << 20) / maxSeconds(uploadElapsed),
		"download":               downloadStats.summarize(downloadElapsed),
		"downloadMiBPerSecond":   float64(downloadBytes.Load()) / (1 << 20) / maxSeconds(downloadElapsed),
		"fileObjectCountForHash": objectCount, "userFileReferencesForHash": referenceCount,
		"uniqueObjectUIDs": len(uniqueUID), "referencesCleaned": *cleanup,
		"correctnessPass": correct, "maximumTestedNotCapacity": true,
	})
}

func uploadGenerated(ctx context.Context, api *apiClient, token, filename string, size int64) (fileMetadata, error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	done := make(chan error, 1)
	go func() {
		part, err := multipartWriter.CreateFormFile("file", filename)
		if err == nil {
			_, err = io.CopyBuffer(part, &generatedReader{remaining: size}, make([]byte, 32*1024))
		}
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
		done <- err
	}()
	envelope, status, err := api.request(ctx, http.MethodPost, "/api/file/file", token, multipartWriter.FormDataContentType(), reader)
	if err != nil {
		_ = reader.CloseWithError(err)
	}
	streamErr := <-done
	if err != nil {
		return fileMetadata{}, err
	}
	if streamErr != nil {
		return fileMetadata{}, streamErr
	}
	if status != http.StatusOK || envelope.Code != 0 {
		return fileMetadata{}, fmt.Errorf("file upload rejected: http=%d code=%d", status, envelope.Code)
	}
	var metadata fileMetadata
	if err := json.Unmarshal(envelope.Data, &metadata); err != nil || metadata.ID == 0 || metadata.UID == "" {
		return fileMetadata{}, fmt.Errorf("file upload returned invalid metadata")
	}
	return metadata, nil
}

func downloadFile(ctx context.Context, api *apiClient, token string, id uint64) (int64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, api.baseURL+"/api/file/"+strconv.FormatUint(id, 10), nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("token", token)
	response, err := api.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("X-Content-Type-Options") != "nosniff" {
		return 0, fmt.Errorf("file download rejected: http=%d", response.StatusCode)
	}
	return io.Copy(io.Discard, response.Body)
}

func generatedSHA256(size int64) string {
	var digest hash.Hash = sha256.New()
	_, _ = io.CopyBuffer(digest, &generatedReader{remaining: size}, make([]byte, 32*1024))
	return hex.EncodeToString(digest.Sum(nil))
}

func maxSeconds(value time.Duration) float64 {
	if value.Seconds() <= 0 {
		return 1
	}
	return value.Seconds()
}
