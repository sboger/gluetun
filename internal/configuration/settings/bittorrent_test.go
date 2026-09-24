package settings

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/qdm12/gosettings/reader"
	"github.com/stretchr/testify/assert"
)

func Test_Bittorrent_setDefaults(t *testing.T) {
	t.Parallel()

	b := Bittorrent{}
	b.setDefaults()

	assert.Equal(t, ptrTo(false), b.Enabled)
	assert.Equal(t, "/downloads", b.DownloadDirectory)
	assert.Equal(t, ptrTo(true), b.DHTEnabled)
	assert.Nil(t, b.Port)
	assert.Nil(t, b.UploadRate)
	assert.Nil(t, b.DownloadRate)
}

func Test_Bittorrent_validate(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		b          Bittorrent
		errMessage string
	}{
		"disabled": {
			b: Bittorrent{Enabled: ptrTo(false)},
		},
		"enabled": {
			b: Bittorrent{
				Enabled:           ptrTo(true),
				DownloadDirectory: "/downloads",
				DHTEnabled:        ptrTo(true),
			},
		},
		"enabled_with_manual_port": {
			b: Bittorrent{
				Enabled:           ptrTo(true),
				DownloadDirectory: "/downloads",
				Port:              ptrTo(uint16(51413)),
				DHTEnabled:        ptrTo(true),
			},
		},
		"empty_download_directory": {
			b: Bittorrent{
				Enabled:    ptrTo(true),
				DHTEnabled: ptrTo(true),
			},
			errMessage: "download directory is empty",
		},
		"zero_port": {
			b: Bittorrent{
				Enabled:           ptrTo(true),
				DownloadDirectory: "/downloads",
				Port:              ptrTo(uint16(0)),
				DHTEnabled:        ptrTo(true),
			},
			errMessage: "port cannot be 0",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := testCase.b.validate()

			if testCase.errMessage != "" {
				assert.ErrorContains(t, err, testCase.errMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Bittorrent_read(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		makeReader func(ctrl *gomock.Controller) *reader.Reader
		b          Bittorrent
	}{
		"defaults": {
			makeReader: func(ctrl *gomock.Controller) *reader.Reader {
				source := newMockSource(ctrl, []sourceKeyValue{
					{key: "BITTORRENT_CLIENT"},
					{key: "BITTORRENT_PORT"},
					{key: "BITTORRENT_DOWNLOAD_DIRECTORY"},
					{key: "BITTORRENT_DHT"},
					{key: "BITTORRENT_UPLOAD_RATE"},
					{key: "BITTORRENT_DOWNLOAD_RATE"},
				})
				return reader.New(reader.Settings{
					Sources: []reader.Source{source},
				})
			},
		},
		"fully_set": {
			makeReader: func(ctrl *gomock.Controller) *reader.Reader {
				source := newMockSource(ctrl, []sourceKeyValue{
					{key: "BITTORRENT_CLIENT", value: "on"},
					{key: "BITTORRENT_PORT", value: "51413"},
					{key: "BITTORRENT_DOWNLOAD_DIRECTORY", value: "/mnt/torrents"},
					{key: "BITTORRENT_DHT", value: "off"},
					{key: "BITTORRENT_UPLOAD_RATE", value: "1024"},
					{key: "BITTORRENT_DOWNLOAD_RATE", value: "2048"},
				})
				return reader.New(reader.Settings{
					Sources: []reader.Source{source},
				})
			},
			b: Bittorrent{
				Enabled:           ptrTo(true),
				Port:              ptrTo(uint16(51413)),
				DownloadDirectory: "/mnt/torrents",
				DHTEnabled:        ptrTo(false),
				UploadRate:        ptrTo(uint64(1024)),
				DownloadRate:      ptrTo(uint64(2048)),
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r := testCase.makeReader(ctrl)

			var b Bittorrent
			err := b.read(r)

			assert.NoError(t, err)
			assert.Equal(t, testCase.b, b)
		})
	}
}

func Test_Bittorrent_String(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		b        Bittorrent
		contains []string
	}{
		"disabled": {
			b:        Bittorrent{Enabled: ptrTo(false)},
			contains: []string{"Enabled: no"},
		},
		"enabled": {
			b: Bittorrent{
				Enabled:           ptrTo(true),
				Port:              ptrTo(uint16(51413)),
				DownloadDirectory: "/downloads",
				DHTEnabled:        ptrTo(true),
			},
			contains: []string{"Enabled: yes", "Port: 51413", "Download directory: /downloads", "DHT: yes"},
		},
		"enabled_auto_port": {
			b: Bittorrent{
				Enabled:           ptrTo(true),
				DownloadDirectory: "/downloads",
				DHTEnabled:        ptrTo(true),
			},
			contains: []string{"Port: auto (VPN forwarded port if enabled)"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := testCase.b.String()

			for _, expected := range testCase.contains {
				assert.Contains(t, s, expected)
			}
		})
	}
}
