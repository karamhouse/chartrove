package host

import (
	"errors"
	"net/url"
	"regexp"
)

var statePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func validateExternalURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil {
		return errors.New("invalid external URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1") {
		return nil
	}
	return errors.New("external URLs require HTTPS (HTTP is allowed only on loopback)")
}

// AuthorizeWindow confines privileged actions to the main window, and callback
// completion to the authentication window created for that particular state.
func AuthorizeWindow(caller, action, state string) error {
	if action == "info" && caller == "main" {
		return nil
	}
	if !statePattern.MatchString(state) {
		return errors.New("invalid authentication window identifier")
	}
	if action == "complete" && caller == "auth-"+state {
		return nil
	}
	if (action == "open" || action == "close") && caller == "main" {
		return nil
	}
	return errors.New("desktop action is not permitted for this window")
}
