package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

func testContract(t *testing.T, s Store) {
	ctx := context.Background()
	key := "households/h1/doc-1"
	data := []byte("%PDF-1.7 hello")

	if _, err := s.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open missing = %v, want ErrNotFound", err)
	}
	if err := s.Put(ctx, key, "application/pdf", bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	rc, err := s.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, data) {
		t.Fatalf("read %q, want %q", got, data)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("second delete = %v, want nil", err)
	}
	if _, err := s.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open after delete = %v, want ErrNotFound", err)
	}
}

func TestLocal(t *testing.T) {
	l, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testContract(t, l)
	for _, bad := range []string{"", "../escape", "a/../../escape"} {
		if err := l.Put(context.Background(), bad, "", bytes.NewReader(nil), 0); err == nil {
			t.Errorf("Put(%q) accepted a key outside the directory", bad)
		}
	}
}

func newFakeS3(t *testing.T) *S3 {
	t.Helper()
	backend := s3mem.New()
	if err := backend.CreateBucket("documents"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(srv.Close)
	return NewS3(S3Config{Endpoint: srv.URL, Region: "ap-southeast-1", Bucket: "documents", AccessKeyID: "test", SecretAccessKey: "test"})
}

func TestS3(t *testing.T) {
	testContract(t, newFakeS3(t))
}

func TestS3PresignedRoundTrip(t *testing.T) {
	s := newFakeS3(t)
	ctx := context.Background()
	data := []byte("\xff\xd8\xff fake jpeg")
	putURL, err := s.PresignPut(ctx, "k/1", "image/jpeg", int64(len(data)), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(putURL, "X-Amz-Signature=") {
		t.Fatalf("not a signed URL: %s", putURL)
	}
	req, _ := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(data))
	req.Header.Set("Content-Type", "image/jpeg")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("presigned PUT status %d", resp.StatusCode)
	}

	getURL, err := s.PresignGet(ctx, "k/1", "image/jpeg", `inline; filename="bill.jpg"`, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.Get(getURL)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.Equal(got, data) {
		t.Fatalf("presigned GET returned %q", got)
	}
}
