// Package domain defines file metadata independently from HTTP multipart and storage implementations.
package domain

import "time"

type FileObject struct {
	ID          uint64
	UID         string
	SHA256      string
	Size        int64
	StoragePath string
	MIMEType    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserFile struct {
	ID           uint64
	UserID       uint64
	FileObjectID uint64
	OriginalName string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type FileMetadata struct {
	UserFile UserFile
	Object   FileObject
}

type UploadResult struct {
	Metadata      FileMetadata
	ObjectCreated bool
	ReferenceNew  bool
}
