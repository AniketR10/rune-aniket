package extension

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"

	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	browserextension "unstable.build/go-tui/api/browser/extension"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	timage "unstable.build/go-tui/component/image"
	"unstable.build/go-tui/component/image/capture"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
)

const defaultFPS = 30

// Greantee returns this extension's extension.Grantee, and it required permissions.
func Grantee() (extension.Grantee, []extension.Permission) {
	webcamGrantee, perms := extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationRight,
		Handler: func(ctx context.Context, cmd textapi.Command,
			grants []extension.Grant, broker proto.MuxBroker,
			invokeWindow browserapi.Window, pconfig config.Config,
		) (browserapi.Handler, error) {
			var publisher term.Interrupter
			for _, grant := range grants {
				if grant.Permission == extension.PermissionBrowserEventPublisher {
					var err error
					publisher, err = browserextension.EventPublisher(ctx, grant, broker)
					if err != nil {
						return nil, fmt.Errorf("browser extension event publisher: %v", err)
					}
				}
			}

			if publisher == nil {
				return nil, errors.New("missing event publisher permission")
			}

			imageConfig := timage.DefaultConfig()
			if color, err := pconfig.GetBool("color"); err != nil {
				if err != config.ErrNotFound {
					log.Warningf("failed to get 'color' from config: %v", err)
				}
			} else {
				imageConfig.Color = color
			}
			if contrast, err := pconfig.GetInt("contrast"); err != nil {
				if err != config.ErrNotFound {
					log.Warningf("failed to get 'contrast' from config: %v", err)
				}
			} else {
				imageConfig.AdjustContrast = float64(contrast)
			}

			fps, err := pconfig.GetInt("fps")
			if err != nil {
				if err != config.ErrNotFound {
					log.Warningf("failed to get 'fps' from config: %v", err)
				}
				fps = defaultFPS
			}
			device, err := capture.NewDevice(publisher, fps, imageConfig)
			if err != nil {
				return nil, fmt.Errorf("new device: %v", err)
			}
			return browserapi.FuncHandler(handler.Nop(device), device.Close), nil
		},
		Command: textapi.CommandManual{
			Name: "rtcGetUserMedia",
			Summary: "Opens up a new window with an ASCII-encoded feed of the user's default" +
				" video input device. This is a prototype that will be evolved into WebRTC " +
				"peer-to-peer calling system.",
		},
	})
	convertImageGrantee, _ := extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationRight,
		Handler: func(ctx context.Context, cmd textapi.Command,
			grants []extension.Grant, broker proto.MuxBroker,
			invokeWindow browserapi.Window, config config.Config) (browserapi.Handler, error) {
			if len(cmd.Args) < 1 {
				return nil, errors.New("expected first argument to be a JPEG image URI")
			}
			img, err := openImage(cmd.Args[0])
			if err != nil {
				return nil, err
			}
			buf := cell.NewBuffer()
			cfg := timage.DefaultConfig()
			cfg.Color = true
			timage.Encode(buf, 120, 60, img, cfg)
			scroll := component.NewScroll(buf)
			return browserapi.NopHandler(handler.Nop(scroll)), nil
		},
		Command: textapi.CommandManual{
			Name: "rtcConvertImageToASCII",
			Summary: "Converts a local JPEG image to an 130x70 ASCII encoded image and opens" +
				" up a window to display it.",
			Synopsis: "image",
		},
	})

	perms = append(perms, extension.PermissionBrowserEventPublisher)
	grantee := extutil.MultiGrantee(webcamGrantee, convertImageGrantee)
	return grantee, perms
}

func openImage(filename string) (image.Image, error) {
	fl, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open: %v", err)
	}
	defer fl.Close()
	img, err := jpeg.Decode(fl)
	if err != nil {
		return nil, fmt.Errorf("jpeg decode: %v", err)
	}
	return img, nil
}
