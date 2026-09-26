package handler

import (
    "bytes"
    "os"
    "testing"
)

func TestAcceptedMediaTaskIDIsPolledWithoutRecreate(t *testing.T) {
    source, err := os.ReadFile("openai_shared_pool_media.go")
    if err != nil {
        t.Fatalf("duplicate accepted media create: read media handler: %v", err)
    }
    if bytes.Contains(source, []byte("ForwardSharedPoolImages(")) || bytes.Contains(source, []byte("ForwardSharedPoolGrokVideo(")) {
        t.Fatal("duplicate accepted media create: accepted upstream task ID is not yet canonical-only")
    }
}