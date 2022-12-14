package main

// FIXME: we should panic less often!

import (
	//"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/BurntSushi/xdg"
	"github.com/aaronjanse/3mux/wm"
)

type UserConfig struct {
	General *CompiledConfigGeneral
	Keys    map[string][]string               `toml:"keys"`
	Modes   map[string]map[string]interface{} `toml:"modes"`
}

type CompiledConfig struct {
	modeStarters map[string]string // key -> mode name
	isSticky     map[string]bool

	normalBindings map[string]func(*wm.Universe)
	modeBindings   map[string]map[string]func(*wm.Universe)

	generalSettings *CompiledConfigGeneral
}

type CompiledConfigGeneral struct {
	EnableHelpBar   bool `toml:"enable-help-bar"`
	EnableStatusBar bool `toml:"enable-status-bar"`
}

func loadOrGenerateConfig() (*CompiledConfig, error) {
	var userTOML string
	//firstRun := false

	xdgConfigPath, err := xdg.Paths{XDGSuffix: "3mux"}.ConfigFile("config.toml")
	if err != nil {
	//	firstRun = true

		usr, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("Failed to get current user: %s", err)
		}
		dirPath := filepath.Join(usr.HomeDir, ".config", "3mux")
		os.MkdirAll(dirPath, os.ModePerm)

		configPath := filepath.Join(dirPath, "config.toml")
		if _, err := os.Stat(configPath); err != nil {
			if os.IsNotExist(err) {
				userTOML = defaultConfig
				ioutil.WriteFile(configPath, []byte(defaultConfig), 0664)
			} else {
				return nil, fmt.Errorf("Failed to read config at `%s`: %s", configPath, err)
			}
		} else {
			return nil, fmt.Errorf("Found in home but not XDG? %s", err)
		}
	} else {
		data, err := ioutil.ReadFile(xdgConfigPath)
		if err != nil {
			return nil, fmt.Errorf("Failed to read config at `%s`: %s", xdgConfigPath, err)
		}
		userTOML = string(data)
	}

	conf := new(UserConfig)
	conf.General = new(CompiledConfigGeneral)

	if _, err := toml.Decode(userTOML, &conf); err != nil {
		return nil, fmt.Errorf("Failed to parse config TOML: %s", err)
	}

	//conf.General.EnableHelpBar = conf.General.EnableHelpBar || firstRun
	conf.General.EnableHelpBar = conf.General.EnableHelpBar //GS

	return compileConfig(conf)
}

func compileConfig(user *UserConfig) (*CompiledConfig, error) {
	conf := &CompiledConfig{
		modeStarters:   map[string]string{},
		isSticky:       map[string]bool{},
		normalBindings: map[string]func(*wm.Universe){},
		modeBindings:   map[string]map[string]func(*wm.Universe){},
	}
	for modeName, mode := range user.Modes {
		sticky, ok := mode["mode-sticky"]
		if ok {
			delete(mode, "mode-sticky")
		} else {
			sticky = false
		}
		conf.isSticky[modeName] = sticky.(bool)

		if starters, ok := mode["mode-start"]; ok {
			switch x := starters.(type) {
			case []interface{}:
				for _, starter := range x {
					starter := strings.ToLower(starter.(string))
					conf.modeStarters[starter] = modeName
				}
			default:
				return nil, fmt.Errorf("Expected []string: %+v (%s)", x, reflect.TypeOf(x))
			}
			delete(mode, "mode-start")
		} else {
			return nil, fmt.Errorf("Could not find starter for mode %s", modeName)
		}

		mode := castMapInterface(mode)
		conf.modeBindings[modeName] = compileBindings(mode)
	}

	conf.normalBindings = compileBindings(user.Keys)

	conf.generalSettings = user.General

	return conf, nil
}

func castMapInterface(source map[string]interface{}) map[string][]string {
	out := map[string][]string{}
	for k, v := range source {
		switch x := v.(type) {
		case []interface{}:
			tmp := []string{}
			for _, abc := range x {
				tmp = append(tmp, abc.(string))
			}
			out[k] = tmp
		default:
			log.Println("Could not cast config", k, v)
		}
	}
	return out
}

func compileBindings(sourceBindings map[string][]string) map[string]func(*wm.Universe) {
	compiledBindings := map[string]func(*wm.Universe){}
	for funcName, keyCodes := range sourceBindings {
		fn, ok := wm.FuncNames[funcName]
		if !ok {
			//GS
			//panic(errors.New("Incorrect keybinding: " + funcName))
			log.Println("Incorrect keybinding:", funcName)
			continue

		}
		for _, keyCode := range keyCodes {
			compiledBindings[strings.ToLower(keyCode)] = fn
		}
	}

	return compiledBindings
}

var mode = ""

func seiveConfigEvents(config *CompiledConfig, u *wm.Universe, human string) bool {
	hu := strings.ToLower(human)
	if mode == "" {
		for key, theMode := range config.modeStarters {
			if hu == key {
				mode = theMode
				return true
			}
		}

		if fn, ok := config.normalBindings[hu]; ok {
			fn(u)
			return true
		}
	} else {
		bindings := config.modeBindings[mode]

		if !config.isSticky[mode] {
			mode = ""
		}

		if fn, ok := bindings[hu]; ok {
			fn(u)
			return true
		}

		mode = ""
	}
	return false
}

const defaultConfig = `[general]

enable-help-bar = false
enable-status-bar = true

[keys]

new-pane  = ['Alt+n', 'Alt+Enter']
kill-pane = ['Alt+q']

all-pane-kill = ['Alt+Shift+Q']

#all-session-kill-quit     = ['Clt+q']
#all-session-keep-detach   = ['Clt+x']

toggle-fullscreen = ['Alt+f']
toggle-search = ['Alt+/']

hide-help-bar = ['Alt+\']

split-pane-vert  = ['Alt+v']
split-pane-horiz = ['ALt+h']

move-pane-up    = ['Alt+Shift+Up'    ]
move-pane-down  = ['Alt+Shift+Down'  ]
move-pane-left  = ['Alt+Shift+Left'  ]
move-pane-right = ['Alt+Shift+Right' ]

move-selection-up    = ['Alt+Up'    ]
move-selection-down  = ['Alt+Down'  ]
move-selection-left  = ['Alt+Left'  ]
move-selection-right = ['Alt+Right' ]

[modes.resize]
mode-start  = ['Alt+R']
mode-sticky = true

resize-up    = ['Up',    'j']
resize-down  = ['Down',  'k']
resize-left  = ['Left',  'h']
resize-right = ['Right', 'l']


`
