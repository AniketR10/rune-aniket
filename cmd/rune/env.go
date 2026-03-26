// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"strings"

	"github.com/Xuanwo/go-locale"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

const fallbackLocale = "UTF-8"

var darwinRe = regexp.MustCompile("UserShell: (/[^ ]+)\n")

func setEnvForGUI(dataPath string) {
	os.Setenv("RUNE_DATADIR", dataPath)

	// set vte vars
	os.Setenv("TERM", "xterm-256color")
	os.Setenv("COLORTERM", "truecolor")

	// https://specifications.freedesktop.org/startup-notification-spec/startup-notification-0.1.txt
	os.Unsetenv("DESKTOP_STARTUP_ID")
	// https://wayland.app/protocols/xdg-activation-v1
	os.Unsetenv("XDG_ACTIVATION_TOKEN")

	if os.Getenv("SHELL") == "" {
		shell, err := userShell()
		if err != nil {
			log.Errorf("shell detect: %v", err)
			shell = "bash"
		}
		os.Setenv("SHELL", shell)
	}

	u, err := user.Current()
	if err == nil {
		os.Setenv("USER", u.Username)
		os.Setenv("HOME", u.HomeDir)
	} else {
		log.Errorf("user detect: %v", err)
	}

	tag, err := locale.Detect()
	if err != nil {
		log.Errorf("locale detect: %v", err)
	} else {
		base, baseConfidence := tag.Base()
		region, regionConfidence := tag.Region()
		if baseConfidence != language.No && regionConfidence != language.No {
			value := fmt.Sprintf("%s_%s.UTF-8", base.String(), region.String())
			log.Debugf("setting LC_ALL to %q", value)
			os.Setenv("LC_ALL", value)
			return
		}
	}

	log.Debugf("setting LC_CTYPE to %q", fallbackLocale)
	os.Setenv("LC_CTYPE", fallbackLocale)
}

func userShell() (string, error) {
	switch runtime.GOOS {
	case "plan9":
		return plan9Shell()
	case "linux":
		return nixShell()
	case "openbsd":
		return nixShell()
	case "freebsd":
		return nixShell()
	case "darwin":
		return darwinShell()
	case "windows":
		return windowsShell()
	}
	return "", fmt.Errorf("undefined GOOS: %s", runtime.GOOS)
}

func plan9Shell() (string, error) {
	if _, err := os.Stat("/dev/osversion"); err != nil {
		if os.IsNotExist(err) {
			return "", err
		} else {
			return "", errors.New("/dev/osversion check failed")
		}
	}

	return "/bin/rc", nil
}

func nixShell() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}

	out, err := exec.Command("getent", "passwd", user.Uid).Output()
	if err != nil {
		return "", err
	}

	ent := strings.Split(strings.TrimSuffix(string(out), "\n"), ":")
	return ent[6], nil
}

func darwinShell() (string, error) {
	dir := "Local/Default/Users/" + os.Getenv("USER")
	out, err := exec.Command("dscl", "localhost", "-read", dir, "UserShell").Output()
	if err != nil {
		return "", err
	}

	matched := darwinRe.FindStringSubmatch(string(out))
	shell := matched[1]
	if shell == "" {
		return "", fmt.Errorf("invalid output: %s", string(out))
	}

	return shell, nil
}

func windowsShell() (string, error) {
	consoleApp := os.Getenv("COMSPEC")
	if consoleApp == "" {
		consoleApp = "cmd.exe"
	}

	return consoleApp, nil
}
