// Copyright (c) 2020 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pushrules_test

import (
	"github.com/stretchr/testify/assert"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/pushrules"

	"testing"
)

func TestPushRule_Match_Conditions(t *testing.T) {
	cond1 := newMatchPushCondition("content.msgtype", "m.emote")
	cond2 := newMatchPushCondition("content.body", "*pushrules")
	rule := &pushrules.PushRule{
		Type:       pushrules.OverrideRule,
		Enabled:    true,
		Conditions: []*pushrules.PushCondition{cond1, cond2},
	}

	evt := newFakeEvent(event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgEmote,
		Body:    "is testing pushrules",
	})
	assert.True(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Conditions_Disabled(t *testing.T) {
	cond1 := newMatchPushCondition("content.msgtype", "m.emote")
	cond2 := newMatchPushCondition("content.body", "*pushrules")
	rule := &pushrules.PushRule{
		Type:       pushrules.OverrideRule,
		Enabled:    false,
		Conditions: []*pushrules.PushCondition{cond1, cond2},
	}

	evt := newFakeEvent(event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgEmote,
		Body:    "is testing pushrules",
	})
	assert.False(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Conditions_FailIfOneFails(t *testing.T) {
	cond1 := newMatchPushCondition("content.msgtype", "m.emote")
	cond2 := newMatchPushCondition("content.body", "*pushrules")
	rule := &pushrules.PushRule{
		Type:       pushrules.OverrideRule,
		Enabled:    true,
		Conditions: []*pushrules.PushCondition{cond1, cond2},
	}

	evt := newFakeEvent(event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    "I'm testing pushrules",
	})
	assert.False(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Content(t *testing.T) {
	rule := &pushrules.PushRule{
		Type:    pushrules.ContentRule,
		Enabled: true,
		Pattern: "is testing*",
	}

	evt := newFakeEvent(event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgEmote,
		Body:    "is testing pushrules",
	})
	assert.True(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Content_Fail(t *testing.T) {
	rule := &pushrules.PushRule{
		Type:    pushrules.ContentRule,
		Enabled: true,
		Pattern: "is testing*",
	}

	evt := newFakeEvent(event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgEmote,
		Body:    "is not testing pushrules",
	})
	assert.False(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Content_ImplicitGlob(t *testing.T) {
	rule := &pushrules.PushRule{
		Type:    pushrules.ContentRule,
		Enabled: true,
		Pattern: "testing",
	}

	evt := newFakeEvent(event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgEmote,
		Body:    "is not testing pushrules",
	})
	assert.True(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Content_IllegalGlob(t *testing.T) {
	rule := &pushrules.PushRule{
		Type:    pushrules.ContentRule,
		Enabled: true,
		Pattern: "this is not a valid glo[b",
	}

	evt := newFakeEvent(event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgEmote,
		Body:    "this is not a valid glob",
	})
	assert.False(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Room(t *testing.T) {
	rule := &pushrules.PushRule{
		Type:    pushrules.RoomRule,
		Enabled: true,
		RuleID:  "!fakeroom:maunium.net",
	}

	evt := newFakeEvent(event.EventMessage, &struct{}{})
	assert.True(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Room_Fail(t *testing.T) {
	rule := &pushrules.PushRule{
		Type:    pushrules.RoomRule,
		Enabled: true,
		RuleID:  "!otherroom:maunium.net",
	}

	evt := newFakeEvent(event.EventMessage, &struct{}{})
	assert.False(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Sender(t *testing.T) {
	rule := &pushrules.PushRule{
		Type:    pushrules.SenderRule,
		Enabled: true,
		RuleID:  "@tulir:maunium.net",
	}

	evt := newFakeEvent(event.EventMessage, &struct{}{})
	assert.True(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_Sender_Fail(t *testing.T) {
	rule := &pushrules.PushRule{
		Type:    pushrules.RoomRule,
		Enabled: true,
		RuleID:  "@someone:matrix.org",
	}

	evt := newFakeEvent(event.EventMessage, &struct{}{})
	assert.False(t, rule.Match(blankTestRoom, evt))
}

func TestPushRule_Match_UnknownTypeAlwaysFail(t *testing.T) {
	rule := &pushrules.PushRule{
		Type:    pushrules.PushRuleType("foobar"),
		Enabled: true,
		RuleID:  "@someone:matrix.org",
	}

	evt := newFakeEvent(event.EventMessage, &struct{}{})
	assert.False(t, rule.Match(blankTestRoom, evt))
}
