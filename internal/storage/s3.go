package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	algorithm     = "AWS4-HMAC-SHA256"
	service       = "s3"
	signedHeaders = "host;x-amz-content-sha256;x-amz-date"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type Client struct {
	Endpoint  string
	PublicURL string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

func (c *Client) Put(ctx context.Context, key, contentType string, body []byte) error {
	return c.do(ctx, http.MethodPut, key, contentType, body)
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.do(ctx, http.MethodDelete, key, "", nil)
}

func (c *Client) do(ctx context.Context, method, key, contentType string, body []byte) error {
	path := "/" + c.Bucket + "/" + encodePath(key)

	req, err := http.NewRequestWithContext(ctx, method, c.Endpoint+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	c.sign(req, path, hashHex(body), time.Now().UTC())

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("storage: %s %s: %s: %s", method, path, res.Status, bytes.TrimSpace(detail))
	}

	return nil
}

func (c *Client) sign(req *http.Request, path, payloadHash string, now time.Time) {
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	canonicalHeaders := "host:" + req.URL.Host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"

	canonicalRequest := strings.Join([]string{
		req.Method,
		path,
		"",
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := dateStamp + "/" + c.Region + "/" + service + "/aws4_request"

	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		scope,
		hashHex([]byte(canonicalRequest)),
	}, "\n")

	key := hmacSum([]byte("AWS4"+c.SecretKey), dateStamp)
	key = hmacSum(key, c.Region)
	key = hmacSum(key, service)
	key = hmacSum(key, "aws4_request")

	req.Header.Set("Authorization", algorithm+
		" Credential="+c.AccessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+
		", Signature="+hex.EncodeToString(hmacSum(key, stringToSign)))
}

func encodePath(path string) string {
	var b strings.Builder

	for i := 0; i < len(path); i++ {
		ch := path[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '-', ch == '_', ch == '.', ch == '~', ch == '/':
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}

	return b.String()
}

func hmacSum(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func hashHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
