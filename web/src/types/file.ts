export interface UserFileRecord {
  id: number
  uid: string
  fileName: string
  size: number
  mimeType: string
  sha256: string
  url: string
  createdAt: string
}

export interface FileUploadResult extends UserFileRecord {
  objectCreated: boolean
  referenceCreated: boolean
  src: string
}

export interface UserFilePage {
  list: UserFileRecord[]
  nextBeforeId: number
}

export type FileUploadStatus = 'waiting' | 'uploading' | 'success' | 'failed'

export interface FileUploadTask {
  id: string
  fileName: string
  size: number
  progress: number
  status: FileUploadStatus
  errorMessage?: string
  userFileID?: number
}
