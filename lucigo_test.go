// Copyright (c) 2024 anabrid GmbH
// Contact: https://www.anabrid.com/licensing/
// SPDX-License-Identifier: MIT OR GPL-2.0-or-later

package lucigo

import (
	"reflect"
	"testing"
)

type TestCandidates struct {
	input  string
	output interface{}
}

var valid_candidates = []TestCandidates{
	{"tcp://1.2.3.4", TCPEndpoint{"1.2.3.4", 5732}},
	{"tcp://1.2.3.4:123", TCPEndpoint{"1.2.3.4", 123}},
	{"serial://dev/null", SerialEndpoint{"/dev/null"}},
	{"serial://COM1", SerialEndpoint{"COM1"}},
}

var known_failures = []string{
	"tcp:/1.2.3.4:123",
	"serial:/dev/null",
	"serial:///dev/null",
}

func TestParseEndpoint_valid_candidates(t *testing.T) {
	for i, test := range valid_candidates {
		endpoint, err := ParseEndpoint(test.input)
		if err != nil {
			t.Fatalf("ParseEndpoint Error at parsing test %d, url %s: %v", i, test.input, err)
		}
		if !reflect.DeepEqual(endpoint, test.output) {
			t.Fatalf(`test %d: ParseEndpoint("%v"), expected %T%#v, got %T%#v`, i, test.input, test.output, test.output, endpoint, endpoint)
		}
	}
}

func TestParseEndpoint_known_failures(t *testing.T) {
	for i, test := range known_failures {
		endpoint, err := ParseEndpoint(test)
		if err == nil {
			t.Fatalf("ParseEndpoint expected error at parsing test %d, url %s but got %#v", i, test, endpoint)
		}
	}
}

func TestParseEndpoint_single(t *testing.T) {
	endpoint, err := ParseEndpoint("tcp://1.2.3.4")
	if err != nil {
		t.Fatalf("ParseEndpoint Error: %v", err)
	}
	if !reflect.DeepEqual(endpoint, TCPEndpoint{"1.2.3.4", 5732}) {
		t.Fatalf(`ParseEndpoint("tcp://1.2.3.4") != JSONLEndpoint{"1.2.3.4", 5732}`)
	}
}
