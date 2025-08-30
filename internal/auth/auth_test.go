package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXUIAuth_Login_Success(t *testing.T) {
	// Mock 3x-ui login interface
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/login", r.URL.Path)

		// Verify Content-Type
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		// Read request body
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		bodyStr := string(body)

		// Verify login parameters
		assert.Contains(t, bodyStr, "username=admin")
		assert.Contains(t, bodyStr, "password=password123")

		// 设置响应头，包含session cookie
		http.SetCookie(w, &http.Cookie{
			Name:  "session",
			Value: "test-session-token",
		})

		// 返回成功响应
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true, "msg": ""}`))
	}))
	defer server.Close()

	// 创建认证客户端
	auth := NewXUIAuth(server.URL, "admin", "password123")

	// 测试登录
	err := auth.Login()
	require.NoError(t, err)

	// 验证session是否被保存
	assert.True(t, auth.IsAuthenticated())
	assert.Equal(t, "test-session-token", auth.GetSessionToken())
}

func TestXUIAuth_Login_InvalidCredentials(t *testing.T) {
	// 模拟3x-ui登录失败
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": false, "msg": "用户名或密码错误"}`))
	}))
	defer server.Close()

	auth := NewXUIAuth(server.URL, "admin", "wrongpassword")
	err := auth.Login()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "用户名或密码错误")
	assert.False(t, auth.IsAuthenticated())
}

func TestXUIAuth_Login_ServerError(t *testing.T) {
	// 模拟服务器错误
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	auth := NewXUIAuth(server.URL, "admin", "password123")
	err := auth.Login()

	assert.Error(t, err)
	assert.False(t, auth.IsAuthenticated())
}

func TestXUIAuth_GetAuthenticatedRequest(t *testing.T) {
	// 先进行登录
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{
				Name:  "session",
				Value: "test-session-token",
			})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "msg": ""}`))
		}
	}))
	defer server.Close()

	auth := NewXUIAuth(server.URL, "admin", "password123")
	err := auth.Login()
	require.NoError(t, err)

	// 测试获取认证请求
	req, err := auth.GetAuthenticatedRequest("GET", "/server/status", nil)
	require.NoError(t, err)

	// 验证请求包含session cookie
	cookies := req.Cookies()
	found := false
	for _, cookie := range cookies {
		if cookie.Name == "session" && cookie.Value == "test-session-token" {
			found = true
			break
		}
	}
	assert.True(t, found, "Request should contain session cookie")
}

func TestXUIAuth_GetAuthenticatedRequest_NotAuthenticated(t *testing.T) {
	auth := NewXUIAuth("http://localhost:54321", "admin", "password123")

	// 测试未认证时获取请求
	_, err := auth.GetAuthenticatedRequest("GET", "/server/status", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
}

func TestXUIAuth_RefreshSession(t *testing.T) {
	loginCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			loginCount++
			sessionValue := "test-session-token"
			if loginCount > 1 {
				sessionValue = "new-session-token"
			}

			http.SetCookie(w, &http.Cookie{
				Name:  "session",
				Value: sessionValue,
			})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "msg": ""}`))
		}
	}))
	defer server.Close()

	auth := NewXUIAuth(server.URL, "admin", "password123")

	// 第一次登录
	err := auth.Login()
	require.NoError(t, err)
	assert.Equal(t, "test-session-token", auth.GetSessionToken())

	// 刷新session
	err = auth.RefreshSession()
	require.NoError(t, err)
	assert.Equal(t, "new-session-token", auth.GetSessionToken())
	assert.Equal(t, 2, loginCount)
}

func TestXUIAuth_IsSessionExpired(t *testing.T) {
	auth := NewXUIAuth("http://localhost:54321", "admin", "password123")

	// 未认证时应该被认为是过期的
	assert.True(t, auth.IsSessionExpired())

	// 设置一个过期的session
	auth.sessionToken = "test-token"
	auth.lastLogin = time.Now().Add(-2 * time.Hour) // 2小时前登录
	assert.True(t, auth.IsSessionExpired())

	// 设置一个有效的session
	auth.lastLogin = time.Now().Add(-30 * time.Minute) // 30分钟前登录
	assert.False(t, auth.IsSessionExpired())
}
