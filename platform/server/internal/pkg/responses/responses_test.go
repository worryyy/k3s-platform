package responses

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/worryyy/k3s-platform/platform/server/internal/pkg/bizerr"
)

func TestSuccessRespData(t *testing.T) {
	c, recorder := testContext()

	Success.RespData(c, gin.H{"id": "rel-1"})

	assertStatus(t, recorder, http.StatusOK)
	response := decodeResponse(t, recorder)
	if response.Code != http.StatusOK {
		t.Fatalf("Code = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Message != "" {
		t.Fatalf("Message = %q, want empty", response.Message)
	}
	data, ok := response.Data.(map[string]interface{})
	if !ok || data["id"] != "rel-1" {
		t.Fatalf("Data = %#v", response.Data)
	}
}

func TestSuccessRespMessage(t *testing.T) {
	c, recorder := testContext()

	Success.RespMessage(c, "created")

	assertStatus(t, recorder, http.StatusOK)
	response := decodeResponse(t, recorder)
	if response.Message != "created" {
		t.Fatalf("Message = %q, want created", response.Message)
	}
	if response.Data != nil {
		t.Fatalf("Data = %#v, want nil", response.Data)
	}
}

func TestSuccessRespDataUsesPresetHTTPStatus(t *testing.T) {
	c, recorder := testContext()

	c.Status(http.StatusAccepted)
	Success.RespData(c, []string{"Queued"})

	assertStatus(t, recorder, http.StatusAccepted)
	response := decodeResponse(t, recorder)
	if response.Code != http.StatusAccepted {
		t.Fatalf("Code = %d, want %d", response.Code, http.StatusAccepted)
	}
	data, ok := response.Data.([]interface{})
	if !ok || len(data) != 1 || data[0] != "Queued" {
		t.Fatalf("Data = %#v", response.Data)
	}
}

func TestFailUsesBizError(t *testing.T) {
	c, recorder := testContext()

	Fail(c, bizerr.NotFound("release not found"))

	assertStatus(t, recorder, http.StatusNotFound)
	response := decodeResponse(t, recorder)
	if response.Code != http.StatusNotFound {
		t.Fatalf("Code = %d, want %d", response.Code, http.StatusNotFound)
	}
	if response.Message != "release not found" {
		t.Fatalf("Message = %q", response.Message)
	}
	if response.Data != nil {
		t.Fatalf("Data = %#v, want nil", response.Data)
	}
}

func TestFailDefaultsUnknownErrors(t *testing.T) {
	c, recorder := testContext()

	Fail(c, errors.New("database password leaked"))

	assertStatus(t, recorder, http.StatusInternalServerError)
	response := decodeResponse(t, recorder)
	if response.Message != "internal server error" {
		t.Fatalf("Message = %q", response.Message)
	}
}

func TestFailMessageOverridesMessageOnly(t *testing.T) {
	c, recorder := testContext()

	FailMessage(c, bizerr.Biz("release rejected"), "custom rejection")

	assertStatus(t, recorder, http.StatusBadRequest)
	response := decodeResponse(t, recorder)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("Code = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response.Message != "custom rejection" {
		t.Fatalf("Message = %q", response.Message)
	}
}

func testContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	return c, recorder
}

func assertStatus(t *testing.T, recorder *httptest.ResponseRecorder, status int) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("HTTP status = %d, want %d; body = %s", recorder.Code, status, recorder.Body.String())
	}
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) Response {
	t.Helper()
	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, recorder.Body.String())
	}
	return response
}
