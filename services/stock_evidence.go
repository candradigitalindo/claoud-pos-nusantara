package services

import (
	"bytes"
	"cloud-pos/database"
	"cloud-pos/models"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

// The timestamp denotes server recording time, not a claim about EXIF capture
// time. Clients cannot choose the timestamp or the uploader displayed on it.
func stampStockEvidence(data []byte, id, actor string, now time.Time) ([]byte, error) {
	if len(data) == 0 || len(data) > 5*1024*1024 {
		return nil, Invalid("foto maksimal 5 MB")
	}
	conf, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") || conf.Width < 1 || conf.Height < 1 || int64(conf.Width)*int64(conf.Height) > 25000000 {
		return nil, Invalid("foto harus JPG, PNG, atau WebP yang valid, maksimal 25 megapiksel")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, Invalid("foto tidak dapat dibaca")
	}
	w, h := conf.Width, conf.Height
	if w > 1600 {
		h = h * 1600 / w
		w = 1600
	}
	if h > 1600 {
		w = w * 1600 / h
		h = 1600
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	width := w
	if width < 720 {
		width = 720
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, h+80))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.RGBA{24, 32, 44, 255}), image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, image.Rect((width-w)/2, 0, (width+w)/2, h), src, src.Bounds(), draw.Src, nil)
	strip := image.NewRGBA(image.Rect(0, 0, width/2, 40))
	draw.Draw(strip, strip.Bounds(), image.NewUniform(color.RGBA{24, 32, 44, 255}), image.Point{}, draw.Src)
	d := font.Drawer{Dst: strip, Src: image.White, Face: basicfont.Face7x13}
	d.Dot = fixed.P(5, 14)
	d.DrawString("SERVER " + now.UTC().Format("2006-01-02 15:04:05 UTC"))
	if len(actor) > 18 {
		actor = actor[:18]
	}
	d.Dot = fixed.P(5, 31)
	d.DrawString(actor + " | " + id)
	xdraw.NearestNeighbor.Scale(dst, image.Rect(0, h, width, h+80), strip, strip.Bounds(), draw.Src, nil)
	var out bytes.Buffer
	if err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func CreateStockEvidence(warehouse string, data []byte, a StockQualityAccess) (*models.StockEvidence, error) {
	if a.ActorID == "" || a.ActorName == "" {
		return nil, ErrStockQualityAccess
	}
	allowed := false
	for _, stage := range []string{"report", "approve", "execute", "receive", "refund", "transfer_receive"} {
		if _, err := qualityWarehouse(database.DB, warehouse, a, stage); err == nil {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrStockQualityAccess
	}
	id := NewULID()
	now := time.Now().UTC()
	photo, err := stampStockEvidence(data, id, a.ActorName, now)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(UploadsDir, now.Format("2006-01"))
	if err = os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, id+".jpg")
	if err = os.WriteFile(path, photo, 0644); err != nil {
		return nil, err
	}
	url := "/" + filepath.ToSlash(path)
	hash := fmt.Sprintf("%x", sha256.Sum256(photo))
	if _, err = database.DB.Exec(`INSERT INTO stock_evidence(id,warehouse_id,url,sha256,actor_id,actor_name,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, warehouse, url, hash, a.ActorID, a.ActorName, now); err != nil {
		os.Remove(path)
		return nil, err
	}
	return &models.StockEvidence{ID: id, URL: url, RecordedAt: now.Format(time.RFC3339), ActorName: a.ActorName}, nil
}
