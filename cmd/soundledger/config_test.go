package main

import "testing"

func TestResolveAddress(t *testing.T) {
	t.Setenv("PORT", "20123")
	address, err := resolveAddress(defaultAddress, false)
	if err != nil {
		t.Fatal(err)
	}
	if address != "127.0.0.1:20123" {
		t.Fatalf("PORT 未生效: %s", address)
	}
	address, err = resolveAddress("127.0.0.1:20444", true)
	if err != nil || address != "127.0.0.1:20444" {
		t.Fatalf("显式地址未生效: %s %v", address, err)
	}
	if _, err = resolveAddress("0.0.0.0:19081", true); err == nil {
		t.Fatal("应拒绝非回环监听")
	}
}
