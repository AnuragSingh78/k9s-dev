// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of K9s

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmeraldMouseEnabledByDefault(t *testing.T) {
	assert.True(t, (UI{}).EmeraldMouseEnabled())
	assert.True(t, (UI{EnableMouse: false}).EmeraldMouseEnabled())
	assert.True(t, (UI{EnableMouse: true}).EmeraldMouseEnabled())
}

func TestEmeraldMouseExplicitOptOutAndOptIn(t *testing.T) {
	off, on := false, true
	assert.False(t, (UI{EmeraldMouse: &off}).EmeraldMouseEnabled())
	assert.True(t, (UI{EmeraldMouse: &on}).EmeraldMouseEnabled())
	assert.False(t, (UI{EnableMouse: true, EmeraldMouse: &off}).EmeraldMouseEnabled())
}
