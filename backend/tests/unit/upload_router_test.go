package unit

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"electron-go-app/backend/internal/handler"
	"electron-go-app/backend/internal/server"
	"github.com/gin-gonic/gin"
)

// TestUploadAvatarWithoutAuth 验证头像上传接口在未登录时仍可成功。
func TestUploadAvatarWithoutAuth(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	uploadHandler := handler.NewUploadHandler(tmpDir)

	router := server.NewRouter(server.RouterOptions{
		UploadHandler: uploadHandler,
		AuthMW:        &failingAuthenticator{t: t},
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="avatar"; filename="avatar.png"`)
	header.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write([]byte{0x89, 0x50, 0x4e, 0x47}); err != nil {
		t.Fatalf("write multipart payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/uploads/avatar", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d, body: %s", resp.Code, resp.Body.String())
	}
}

// failingAuthenticator 会在中间件被触发时让单测失败，确保上传路由未挂载鉴权。
type failingAuthenticator struct {
	t *testing.T
}

// Handle 会在被调用时直接让测试失败，证明请求没有经过鉴权链路。
func (f *failingAuthenticator) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		f.t.Fatalf("auth middleware should not be invoked for uploads")
	}
}
