package handlers

import (
	"net/http"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/color"
)

type RGBResp struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

type HSLResp struct {
	H int `json:"h"`
	S int `json:"s"`
	L int `json:"l"`
}

type HSVResp struct {
	H int `json:"h"`
	S int `json:"s"`
	V int `json:"v"`
}

type OkLChResp struct {
	L float64 `json:"l"`
	C float64 `json:"c"`
	H int     `json:"h"`
}

type ContrastResp struct {
	Ratio float64 `json:"ratio"`
	Level string  `json:"level"`
}

type ContrastsResp struct {
	White ContrastResp `json:"white"`
	Black ContrastResp `json:"black"`
}

type ColorInfoResp struct {
	Hex      string        `json:"hex"`
	RGB      RGBResp       `json:"rgb"`
	HSL      HSLResp       `json:"hsl"`
	HSV      HSVResp       `json:"hsv"`
	OkLCh    OkLChResp     `json:"oklch"`
	Contrast ContrastsResp `json:"contrast"`
}

func HandleColorInfo(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	hex, err := hexFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	w.Header().Set("Cache-Control", staticCacheControl)
	httputils.WriteJSON(ctx, w, http.StatusOK, colorInfoResp(color.Describe(hex)))
}

func colorInfoResp(info color.Info) ColorInfoResp {
	return ColorInfoResp{
		Hex:   string(info.Hex),
		RGB:   RGBResp{R: info.RGB.R, G: info.RGB.G, B: info.RGB.B},
		HSL:   HSLResp{H: info.HSL.Hue, S: info.HSL.Saturation, L: info.HSL.Lightness},
		HSV:   HSVResp{H: info.HSV.Hue, S: info.HSV.Saturation, V: info.HSV.Value},
		OkLCh: OkLChResp{L: info.OkLCh.Lightness, C: info.OkLCh.Chroma, H: info.OkLCh.Hue},
		Contrast: ContrastsResp{
			White: contrastResp(info.AgainstWhite),
			Black: contrastResp(info.AgainstBlack),
		},
	}
}

func contrastResp(c color.Contrast) ContrastResp {
	return ContrastResp{Ratio: c.Ratio, Level: string(c.Level)}
}
