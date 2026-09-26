package settings

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/qdm12/gosettings/reader"
	"github.com/stretchr/testify/assert"
)

func Test_WebUI_setDefaults(t *testing.T) {
	t.Parallel()

	w := WebUI{}
	w.setDefaults()

	assert.Equal(t, ptrTo(false), w.Enabled)
	assert.Equal(t, ptrTo(uint16(7999)), w.Port)
}

func Test_WebUI_validate(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		w          WebUI
		errMessage string
	}{
		"disabled": {
			w: WebUI{Enabled: ptrTo(false), Port: ptrTo(uint16(7999))},
		},
		"enabled": {
			w: WebUI{Enabled: ptrTo(true), Port: ptrTo(uint16(7999))},
		},
		"zero_port": {
			w:          WebUI{Enabled: ptrTo(true), Port: ptrTo(uint16(0))},
			errMessage: "web UI port cannot be 0",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := testCase.w.validate()

			if testCase.errMessage != "" {
				assert.ErrorContains(t, err, testCase.errMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_WebUI_read(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		makeReader func(ctrl *gomock.Controller) *reader.Reader
		w          WebUI
	}{
		"defaults": {
			makeReader: func(ctrl *gomock.Controller) *reader.Reader {
				source := newMockSource(ctrl, []sourceKeyValue{
					{key: "GLUETUN_WEBUI"},
					{key: "GLUETUN_WEBUI_PORT"},
				})
				return reader.New(reader.Settings{
					Sources: []reader.Source{source},
				})
			},
		},
		"fully_set": {
			makeReader: func(ctrl *gomock.Controller) *reader.Reader {
				source := newMockSource(ctrl, []sourceKeyValue{
					{key: "GLUETUN_WEBUI", value: "on"},
					{key: "GLUETUN_WEBUI_PORT", value: "8888"},
				})
				return reader.New(reader.Settings{
					Sources: []reader.Source{source},
				})
			},
			w: WebUI{
				Enabled: ptrTo(true),
				Port:    ptrTo(uint16(8888)),
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r := testCase.makeReader(ctrl)

			var w WebUI
			err := w.read(r)

			assert.NoError(t, err)
			assert.Equal(t, testCase.w, w)
		})
	}
}
