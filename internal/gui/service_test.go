package gui

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/bluehandle-open/bluehandle-go/internal/config"
	"github.com/bluehandle-open/bluehandle-go/internal/session"
)

type settingsHost struct {
	startup   bool
	calls     []bool
	failures  []error
	refreshes int
}

func (h *settingsHost) SetStartOnBoot(on bool) error {
	h.calls = append(h.calls, on)
	if i := len(h.calls) - 1; i < len(h.failures) && h.failures[i] != nil {
		return h.failures[i]
	}
	h.startup = on
	return nil
}

func (*settingsHost) RestartElevated() error { return nil }
func (h *settingsHost) RefreshLanguage()     { h.refreshes++ }

func TestSaveSettingsStartupTransaction(t *testing.T) {
	registerErr := errors.New("startup registration failed")
	saveErr := errors.New("config write failed")
	rollbackErr := errors.New("startup rollback failed")
	for _, before := range []bool{false, true} {
		for _, tc := range []struct {
			name          string
			changeStartup bool
			failRegister  bool
			failSave      bool
			failRollback  bool
		}{
			{name: "success", changeStartup: true},
			{name: "registration failure", changeStartup: true, failRegister: true},
			{name: "save failure rolls back", changeStartup: true, failSave: true},
			{name: "rollback failure reports both errors", changeStartup: true, failSave: true, failRollback: true},
			{name: "unchanged startup needs no registration"},
			{name: "unchanged startup needs no rollback", failSave: true},
		} {
			t.Run(fmt.Sprintf("initial=%t/%s", before, tc.name), func(t *testing.T) {
				cfg := config.Default()
				cfg.StartOnBoot = before
				host := &settingsHost{startup: before, failures: make([]error, 2)}
				if tc.failRegister {
					host.failures[0] = registerErr
				}
				if tc.failRollback {
					host.failures[1] = rollbackErr
				}
				target := before
				if tc.changeStartup {
					target = !before
				}
				saves := 0
				s := session.New(nil, cfg, func(c config.Config) error {
					saves++
					if host.startup != target || c.StartOnBoot != target || c.Language != config.LanguageEnglish {
						t.Errorf("persisted before startup registration or with stale settings: host=%t config=%+v", host.startup, c)
					}
					if tc.failSave {
						return saveErr
					}
					return nil
				}, nil)
				service := NewService(s, host, "test")
				st := service.Settings()
				st.Language = config.LanguageEnglish
				st.StartOnBoot = target
				st.ShowText = !st.ShowText
				err := service.SaveSettings(st)
				for expected, failure := range map[error]bool{registerErr: tc.failRegister, saveErr: tc.failSave, rollbackErr: tc.failRollback} {
					if errors.Is(err, expected) != failure {
						t.Errorf("error %v: contains %v = %t, want %t", err, expected, errors.Is(err, expected), failure)
					}
				}
				failed := tc.failRegister || tc.failSave
				if (err != nil) != failed {
					t.Fatalf("SaveSettings error = %v, want failure=%t", err, failed)
				}
				wantCfg := cfg
				if !failed {
					wantCfg.Language = st.Language
					wantCfg.StartOnBoot = target
					wantCfg.ShowText = st.ShowText
				}
				if got := s.Config(); !reflect.DeepEqual(got, wantCfg) {
					t.Errorf("in-memory config = %+v, want %+v", got, wantCfg)
				}
				if got := service.Settings(); got.StartOnBoot != wantCfg.StartOnBoot {
					t.Errorf("displayed startup = %t, want %t", got.StartOnBoot, wantCfg.StartOnBoot)
				}
				var wantCalls []bool
				if tc.changeStartup {
					wantCalls = append(wantCalls, target)
					if tc.failSave {
						wantCalls = append(wantCalls, before)
					}
				}
				if !reflect.DeepEqual(host.calls, wantCalls) {
					t.Errorf("startup calls = %v, want %v", host.calls, wantCalls)
				}
				wantStartup := target
				if tc.failRegister || tc.failSave && !tc.failRollback {
					wantStartup = before
				}
				if host.startup != wantStartup {
					t.Errorf("OS startup = %t, want %t", host.startup, wantStartup)
				}
				wantSaves := 1
				if tc.failRegister {
					wantSaves = 0
				}
				if saves != wantSaves {
					t.Errorf("config saves = %d, want %d", saves, wantSaves)
				}
				wantRefreshes := 1
				if failed {
					wantRefreshes = 0
				}
				if host.refreshes != wantRefreshes {
					t.Errorf("language refreshes = %d, want %d", host.refreshes, wantRefreshes)
				}
			})
		}
	}
}
