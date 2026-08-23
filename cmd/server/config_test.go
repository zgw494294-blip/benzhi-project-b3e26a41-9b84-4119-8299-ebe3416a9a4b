package main

import "testing"

func TestParseConfigAddressAndPort(t *testing.T) {
	config, err := parseConfig(nil, func(key string) string {
		if key == "PORT" {
			return "19123"
		}
		return ""
	})
	if err != nil || config.Address != "127.0.0.1:19123" {
		t.Fatalf("unexpected PORT config: %#v err=%v", config, err)
	}
	config, err = parseConfig([]string{"-addr=127.0.0.1:19400"}, func(string) string { return "" })
	if err != nil || config.Address != "127.0.0.1:19400" {
		t.Fatalf("unexpected explicit config: %#v err=%v", config, err)
	}
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19400"}, func(string) string { return "" }); err == nil {
		t.Fatal("expected non-loopback address rejection")
	}
}
