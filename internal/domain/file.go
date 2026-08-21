package domain

// FileStorageBackend 定义文件内容实际保存的位置。
type FileStorageBackend string

const (
	FileStorageBackendLocal FileStorageBackend = "local"
	FileStorageBackendS3    FileStorageBackend = "s3"
)

// FilePurpose 定义文件上传用途。
type FilePurpose string

const (
	FilePurposeUserAvatar FilePurpose = "user_avatar"
)

// FileStatus 定义文件上传状态。
type FileStatus string

const (
	FileStatusPending  FileStatus = "pending"
	FileStatusUploaded FileStatus = "uploaded"
	FileStatusActive   FileStatus = "active"
	FileStatusDeleting FileStatus = "deleting"
)
