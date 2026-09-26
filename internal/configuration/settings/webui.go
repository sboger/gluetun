package settings

import (
	"errors"
	"fmt"
	"os"

	"github.com/qdm12/gosettings"
	"github.com/qdm12/gosettings/reader"
	"github.com/qdm12/gotree"
)

const defaultWebUIPort = 7999

// ErrWebUIPortZero is returned when the web UI is enabled
// with a listening port of 0.
var ErrWebUIPortZero = errors.New("web UI port cannot be 0")

// WebUI contains settings to customize the optional
// web UI server, which exposes a simple browser interface
// on top of the control server API.
type WebUI struct {
	// Enabled is true if the web UI server should run.
	// It cannot be nil in the internal state.
	Enabled *bool
	// Port is the listening port for the web UI server.
	// It cannot be nil in the internal state and defaults to 7999.
	Port *uint16
}

func (w WebUI) validate() (err error) {
	if !*w.Enabled {
		return nil
	}

	if *w.Port == 0 {
		return ErrWebUIPortZero
	}

	uid := os.Getuid()
	const maxPrivilegedPort = 1023
	if uid != 0 && *w.Port <= maxPrivilegedPort {
		return fmt.Errorf("%w: %d when running with user ID %d",
			ErrControlServerPrivilegedPort, *w.Port, uid)
	}

	return nil
}

func (w *WebUI) copy() (copied WebUI) {
	return WebUI{
		Enabled: gosettings.CopyPointer(w.Enabled),
		Port:    gosettings.CopyPointer(w.Port),
	}
}

func (w *WebUI) overrideWith(other WebUI) {
	w.Enabled = gosettings.OverrideWithPointer(w.Enabled, other.Enabled)
	w.Port = gosettings.OverrideWithPointer(w.Port, other.Port)
}

func (w *WebUI) setDefaults() {
	w.Enabled = gosettings.DefaultPointer(w.Enabled, false)
	w.Port = gosettings.DefaultPointer(w.Port, defaultWebUIPort)
}

func (w WebUI) String() string {
	return w.toLinesNode().String()
}

func (w WebUI) toLinesNode() (node *gotree.Node) {
	node = gotree.New("Web UI settings:")
	node.Appendf("Enabled: %s", gosettings.BoolToYesNo(w.Enabled))
	if !*w.Enabled {
		return node
	}
	node.Appendf("Port: %d", *w.Port)
	return node
}

func (w *WebUI) read(r *reader.Reader) (err error) {
	w.Enabled, err = r.BoolPtr("GLUETUN_WEBUI")
	if err != nil {
		return err
	}

	w.Port, err = r.Uint16Ptr("GLUETUN_WEBUI_PORT")
	if err != nil {
		return err
	}

	return nil
}
