package whatsmeow

import (
	"context"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"testing"
)

func TestDeviceStoreEngine(t *testing.T) {
	target := dbtest.Target(t)
	container, err := openDeviceStore(context.Background(), target, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()
	devices, err := container.GetAllDevices(context.Background())
	if err != nil || len(devices) != 0 {
		t.Fatalf("fresh device store: %d devices: %v", len(devices), err)
	}
}
