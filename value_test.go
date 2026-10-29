package main

import "testing"

// TestValueEqual 钉住 Equal 的语义：kind 与内容都相同才相等，int 与 string 不互通。
func TestValueEqual(t *testing.T) {
	if !IntValue(7).Equal(IntValue(7)) {
		t.Error("相同 int 应相等")
	}
	if IntValue(7).Equal(IntValue(8)) {
		t.Error("不同 int 不应相等")
	}
	if IntValue(7).Equal(StringValue("7")) {
		t.Error("int 与 string 不应相等")
	}
	if !StringValue("hi").Equal(StringValue("hi")) {
		t.Error("相同 string 应相等")
	}
	if StringValue("hi").Equal(StringValue("hey")) {
		t.Error("不同 string 不应相等")
	}

	if !BoolValue(true).Equal(BoolValue(true)) {
		t.Error("相同 bool 应相等")
	}
	if BoolValue(true).Equal(BoolValue(false)) {
		t.Error("不同 bool 不应相等")
	}
	if BoolValue(true).Equal(IntValue(1)) {
		t.Error("bool 与 int 不应相等")
	}
}
