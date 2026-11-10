package provider

import (
	"maps"
	"testing"
)

// An update sends the values of names the server doesn't hold, every value
// when resending, and nil for each name the server holds that isn't
// configured.
func TestWebhookHandlerSecretChanges(t *testing.T) {
	configured := map[string]string{"token": "t1", "added": "a1"}
	for name, tc := range map[string]struct {
		serverNames []string
		resendAll   bool
		want        map[string]string
		deleted     []string
	}{
		"nothing changed":    {serverNames: []string{"token", "added"}, want: map[string]string{}},
		"a name added":       {serverNames: []string{"token"}, want: map[string]string{"added": "a1"}},
		"a name removed":     {serverNames: []string{"token", "added", "old"}, want: map[string]string{}, deleted: []string{"old"}},
		"resending":          {serverNames: []string{"token", "added", "old"}, resendAll: true, want: map[string]string{"token": "t1", "added": "a1"}, deleted: []string{"old"}},
		"a server with none": {want: map[string]string{"token": "t1", "added": "a1"}},
	} {
		changes := webhookHandlerSecretChanges(configured, tc.serverNames, tc.resendAll)
		set := map[string]string{}
		var deleted []string
		for name, value := range changes {
			if value == nil {
				deleted = append(deleted, name)
			} else {
				set[name] = *value
			}
		}
		if !maps.Equal(set, tc.want) || len(deleted) != len(tc.deleted) || (len(deleted) == 1 && deleted[0] != tc.deleted[0]) {
			t.Errorf("%s: set %v, deleted %v; want set %v, deleted %v", name, set, deleted, tc.want, tc.deleted)
		}
	}
}
