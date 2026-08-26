package appservice

import "github.com/runforyou-ai/cervi/internal/domain"

// FilePurpose 表示文件上传用途。
type FilePurpose string

const (
	FilePurposeUserAvatar FilePurpose = FilePurpose(domain.FilePurposeUserAvatar)
)

// FileUploadInput 定义创建上传所需的文件元数据。
type FileUploadInput struct {
	Purpose     FilePurpose `json:"purpose"`
	FileName    string      `json:"fileName"`
	ContentType string      `json:"contentType"`
	ByteSize    int64       `json:"byteSize"`
}

// File 定义前端可使用的文件元数据。
type File struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	ByteSize    int64  `json:"byteSize"`
	ContentURL  string `json:"contentUrl"`
}

// FileUploadRequest 定义客户端上传文件内容所需的 HTTP 请求。
type FileUploadRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// FileUpload 包含待上传文件和内容上传请求。
type FileUpload struct {
	File    File              `json:"file"`
	Request FileUploadRequest `json:"request"`
}

// ProfileImageFile 定义原生端选择的用户头像文件。
type ProfileImageFile struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	DataBase64  string `json:"dataBase64"`
}
