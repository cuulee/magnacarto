package mapnik

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestMap(t *testing.T) {
	m := New()
	if err := m.Load("test/map.xml"); err != nil {
		t.Fatal(err)
	}

	m.ZoomAll()
	img, err := m.RenderImage(RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Rect.Dx() != 800 || img.Rect.Dy() != 600 {
		t.Error("unexpected size of output image: ", img.Rect)
	}
}

func TestRenderFile(t *testing.T) {
	m := New()
	if err := m.Load("test/map.xml"); err != nil {
		t.Fatal(err)
	}
	m.ZoomAll()

	out, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("unable to create temp dir")
	}
	defer os.RemoveAll(out)

	fname := filepath.Join(out, "out.png")
	if err := m.RenderToFile(RenderOpts{}, fname); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(fname)
	if err != nil {
		t.Fatal("unable to open test output", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal("unable to open test output", err)
	}
	if img.Bounds().Dx() != 800 || img.Bounds().Dy() != 600 {
		t.Error("unexpected size of output image: ", img.Bounds())
	}
}

func TestRenderInvalidFormat(t *testing.T) {
	m := New()
	if err := m.Load("test/map.xml"); err != nil {
		t.Fatal(err)
	}
	m.ZoomAll()

	if _, err := m.Render(RenderOpts{Format: "invalidformat"}); err == nil {
		t.Fatal("invalid format did not return an error")
	}
}

func TestSRS(t *testing.T) {
	m := New()
	// default mapnil srs
	if srs := m.SRS(); srs != "+proj=longlat +ellps=WGS84 +datum=WGS84 +no_defs" {
		t.Fatal("unexpected default srs:", srs)
	}
	if err := m.Load("test/map.xml"); err != nil {
		t.Fatal(err)
	}
	if m.SRS() != "+init=epsg:4326" {
		t.Error("unexpeced srs: ", m.SRS())
	}
	m.SetSRS("+init=epsg:3857")
	if m.SRS() != "+init=epsg:3857" {
		t.Error("unexpeced srs: ", m.SRS())
	}
}

func TestBackgroundColor(t *testing.T) {
	m := New()
	c := m.BackgroundColor()
	if c.R != 0 || c.G != 0 || c.B != 0 || c.A != 0 {
		t.Error("default background not transparent", c)
	}

	m.SetBackgroundColor(color.NRGBA{100, 50, 200, 150})
	c = m.BackgroundColor()
	if c.R != 100 || c.G != 50 || c.B != 200 || c.A != 150 {
		t.Error("background not set", c)
	}
	img, err := m.RenderImage(RenderOpts{})
	if err != nil {
		t.Fatal(err)
	}
	bg := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
	if bg.R != 98 || bg.G != 49 || bg.B != 198 || bg.A != 150 {
		t.Error("background in rendered image not set", bg)
	}
}

func TestRender(t *testing.T) {
	m := New()
	if err := m.Load("test/map.xml"); err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("unable to create temp dir")
	}
	defer os.RemoveAll(out)

	fname := filepath.Join(out, "out.png")
	opts := RenderOpts{Format: "png24"}

	if err := m.RenderToFile(opts, fname); err != nil {
		t.Fatal(err)
	}
	bufFile, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	bufDirect, err := m.Render(opts)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(bufDirect, bufFile) != 0 {
		t.Error("RenderFile and Render output differs")
	}
}

type testSelector struct {
	status func(string) Status
}

func (t *testSelector) Select(layer string) Status {
	return t.status(layer)
}

func TestLayerStatus(t *testing.T) {
	m := New()
	if err := m.Load("test/map.xml"); err != nil {
		t.Fatal(err)
	}

	if m.layerStatus != nil {
		t.Error("default layer status not nil")
	}

	if !reflect.DeepEqual(m.currentLayerStatus(), []bool{true, true, true, false}) {
		t.Error("unexpected layer status", m.currentLayerStatus())
	}

	m.storeLayerStatus()
	if !reflect.DeepEqual(m.layerStatus, []bool{true, true, true, false}) {
		t.Error("unexpected layer status", m.layerStatus)
	}
	m.resetLayerStatus()

	ts := testSelector{func(layer string) Status {
		if layer == "layerA" {
			return Exclude
		}
		if layer == "layerB" {
			return Include
		}
		return Default
	}}

	m.SelectLayers(&ts)
	if !reflect.DeepEqual(m.layerStatus, []bool{true, true, true, false}) {
		t.Error("unexpected layer status", m.layerStatus)
	}
	if !reflect.DeepEqual(m.currentLayerStatus(), []bool{false, true, true, false}) {
		t.Error("unexpected layer status", m.currentLayerStatus())
	}

	m.ResetLayers()
	if m.layerStatus != nil {
		t.Error("unexpected layer status", m.layerStatus)
	}
	if !reflect.DeepEqual(m.currentLayerStatus(), []bool{true, true, true, false}) {
		t.Error("unexpected layer status", m.currentLayerStatus())
	}

}

func prepareImg(t testing.TB) *image.NRGBA {
	r, err := os.Open("test/encode_test.png")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	img, _, err := image.Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatal("image not NRGBA")
	}
	return nrgba
}

func benchmarkEncodeMapnik(b *testing.B, format, suffix string) {
	img := prepareImg(b)
	for i := 0; i < b.N; i++ {
		_, err := Encode(img, format)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestEncode(t *testing.T) {
	img := prepareImg(t)
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		t.Fatal(err)
	}

	imgGo, _, err := image.Decode(buf)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, img.Bounds(), imgGo.Bounds())

	b, err := Encode(img, "png")
	if err != nil {
		t.Fatal(err)
	}

	imgMapnik, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}

	assertImageEqual(t, img, imgMapnik)
	assertImageEqual(t, imgGo, imgMapnik)
}

func TestEncodeInvalidFormat(t *testing.T) {
	img := prepareImg(t)

	if _, err := Encode(img, "invalid"); err == nil {
		t.Fatal("invalid format did not return an error")
	}
}

func assertImageEqual(t *testing.T, a, b image.Image) {
	assertEqual(t, a.Bounds(), b.Bounds())
	for y := 0; y < a.Bounds().Max.Y; y++ {
		for x := 0; x < a.Bounds().Max.X; x++ {
			assertEqual(t,
				color.RGBAModel.Convert(a.At(x, y)),
				color.RGBAModel.Convert(b.At(x, y)),
			)
		}
	}
}

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if !reflect.DeepEqual(a, b) {
		for i := 0; ; i++ {
			i += 1
			pc, file, line, ok := runtime.Caller(i)
			if !ok {
				break
			}
			if f := runtime.FuncForPC(pc); f != nil && strings.HasPrefix(f.Name(), "assert") {
				continue
			}
			t.Fatalf("%v != %v at: %s:%d", a, b, filepath.Base(file), line)
			return
		}
		t.Fatalf("%v != %v", a, b)
	}
}

func BenchmarkEncodeMapnik(b *testing.B) { benchmarkEncodeMapnik(b, "png256", "png") }

func BenchmarkEncodeMapnikPngHex(b *testing.B)    { benchmarkEncodeMapnik(b, "png256:m=h", "png") }
func BenchmarkEncodeMapnikPngOctree(b *testing.B) { benchmarkEncodeMapnik(b, "png256:m=o", "png") }

func BenchmarkEncodeMapnikJpeg(b *testing.B) { benchmarkEncodeMapnik(b, "jpeg", "jpeg") }

func BenchmarkEncodeGo(b *testing.B) {
	img := prepareImg(b)

	for i := 0; i < b.N; i++ {
		buf := &bytes.Buffer{}
		if err := png.Encode(buf, img); err != nil {
			b.Fatal(err)
		}
	}
}
