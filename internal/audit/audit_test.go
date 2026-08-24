package audit

import (
	"testing"
	"time"
)

func TestCanonicalDigestIgnoresMapInsertionOrder(t *testing.T) {
	left := map[string]any{"z": 1, "a": map[string]any{"b": 2, "a": 1}}
	right := map[string]any{"a": map[string]any{"a": 1, "b": 2}, "z": 1}
	ld, err := Digest(left)
	if err != nil {
		t.Fatal(err)
	}
	rd, err := Digest(right)
	if err != nil {
		t.Fatal(err)
	}
	if ld != rd {
		t.Fatalf("规范摘要不一致: %s %s", ld, rd)
	}
}
func TestEventChainDetectsTampering(t *testing.T) {
	event, err := SealEvent(EventRecord{Sequence: 1, BatchID: "b", BatchVersion: 1, Type: "created", ActorID: "u", RequestID: "r", OccurredAt: time.Unix(1, 0).UTC(), Payload: map[string]any{"ok": true}})
	if err != nil {
		t.Fatal(err)
	}
	if err = VerifyEvent(event, 1, ""); err != nil {
		t.Fatal(err)
	}
	event.Type = "changed"
	if err = VerifyEvent(event, 1, ""); err == nil {
		t.Fatal("篡改事件应校验失败")
	}
}
