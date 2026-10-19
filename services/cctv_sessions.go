package services

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"cloud-pos/models"
)

// Setiap penonton memiliki nama relay sendiri. Menutup satu tab tidak boleh
// menghentikan penonton lain, termasuk ketika kamera yang sama ada di monitor.
type cctvStreamSession struct {
	camera          models.OutletCamera
	used            time.Time
	created         time.Time
	closing         bool
	cleanupFailures int
}

var cctvStreams = struct {
	sync.Mutex
	sessions map[string]cctvStreamSession
}{sessions: map[string]cctvStreamSession{}}

var cctvCameraLocks [64]sync.Mutex
var cctvCleanerOnce sync.Once

const cctvStreamIdleTTL = 3 * time.Minute

func lockCCTVCamera(id string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	lock := &cctvCameraLocks[h.Sum32()%uint32(len(cctvCameraLocks))]
	lock.Lock()
	return lock.Unlock
}

func registerCCTVStream(cam models.OutletCamera, source, mode string) (string, error) {
	prefix := "cam_"
	if mode == "playback" {
		prefix = "pb_"
	}
	name := prefix + cam.ID + "_" + strings.ToLower(NewULID()[18:])
	cctvStreams.Lock()
	if len(cctvStreams.sessions) >= 256 {
		cctvStreams.Unlock()
		return "", fmt.Errorf("%w: batas tayangan bersamaan tercapai; tutup tayangan yang tidak digunakan", ErrCCTVStreamLimit)
	}
	cctvStreams.sessions[name] = cctvStreamSession{camera: cam, used: time.Now(), created: time.Now()}
	cctvStreams.Unlock()
	if err := publishCameraStream(cam, name, source); err != nil {
		cctvStreams.Lock()
		delete(cctvStreams.sessions, name)
		cctvStreams.Unlock()
		return "", err
	}
	return name, nil
}

func managedCCTVStream(name string) bool {
	return strings.HasPrefix(name, "pb_") || len(name) == 39
}

func cameraForCCTVStream(name string) (*models.OutletCamera, error) {
	cctvStreams.Lock()
	session, ok := cctvStreams.sessions[name]
	if ok && !session.closing {
		session.used = time.Now()
		cctvStreams.sessions[name] = session
	}
	cctvStreams.Unlock()
	if ok && !session.closing {
		return &session.camera, nil
	}
	if managedCCTVStream(name) {
		return nil, ErrCCTVAccessDenied
	}
	// Kompatibilitas named-stream live lama; semua sesi baru selalu unik.
	return cctvCameraLoaderName(name)
}

func invalidateCCTVHLS(cameraID, name string) {
	cctvHLS.Lock()
	for id, session := range cctvHLS.sessions {
		streamCamera, _ := CCTVStreamCameraID(session.stream)
		if session.stream == name || cameraID != "" && streamCamera == cameraID {
			delete(cctvHLS.sessions, id)
		}
	}
	cctvHLS.Unlock()
}

// Pemanggil memegang lock kamera. Simpan snapshot relay sampai penghapusan
// berhasil, agar perubahan mode gateway/direct tidak membersihkan relay salah.
func stopCCTVStream(cameraID, name string) error {
	cctvStreams.Lock()
	session, ok := cctvStreams.sessions[name]
	if ok {
		session.closing = true
		cctvStreams.sessions[name] = session
	}
	cctvStreams.Unlock()
	invalidateCCTVHLS("", name)
	var cam *models.OutletCamera
	if ok {
		cam = &session.camera
	} else {
		if managedCCTVStream(name) {
			return nil // stop idempotent, termasuk respons start yang datang terlambat
		}
		var err error
		cam, err = cctvCameraLoader(cameraID)
		if err != nil {
			return err
		}
	}
	if err := deleteCameraStream(*cam, name); err != nil {
		cctvStreams.Lock()
		if current, exists := cctvStreams.sessions[name]; exists {
			current.cleanupFailures++
			if current.cleanupFailures >= 3 {
				delete(cctvStreams.sessions, name)
			} else {
				cctvStreams.sessions[name] = current
			}
		}
		cctvStreams.Unlock()
		return err
	}
	cctvStreams.Lock()
	delete(cctvStreams.sessions, name)
	cctvStreams.Unlock()
	return nil
}

func StopStream(cameraID, name string) error {
	id, err := CCTVStreamCameraID(name)
	if err != nil || id != cameraID {
		return ErrCCTVAccessDenied
	}
	unlock := lockCCTVCamera(cameraID)
	defer unlock()
	return stopCCTVStream(cameraID, name)
}

func invalidateCameraStreams(cam models.OutletCamera) {
	invalidateCCTVHLS(cam.ID, "")
	cctvStreams.Lock()
	for name, session := range cctvStreams.sessions {
		if session.camera.ID == cam.ID {
			session.closing = true
			cctvStreams.sessions[name] = session
		}
	}
	cctvStreams.Unlock()
	// Sesi langsung ditolak. Cleaner menghapus relay di latar belakang agar
	// outlet terputus tidak menahan operasi penyimpanan konfigurasi.
}

func sweepCCTVStreams(now time.Time) {
	cctvStreams.Lock()
	names := map[string]string{}
	for name, session := range cctvStreams.sessions {
		if session.closing || now.Sub(session.used) >= cctvStreamIdleTTL || strings.HasPrefix(name, "pb_") && now.Sub(session.created) >= time.Hour {
			names[name] = session.camera.ID
		}
	}
	cctvStreams.Unlock()
	for name, id := range names {
		unlock := lockCCTVCamera(id)
		// A media request may have refreshed this stream while waiting on the lock.
		cctvStreams.Lock()
		session, ok := cctvStreams.sessions[name]
		expired := ok && (session.closing || now.Sub(session.used) >= cctvStreamIdleTTL || strings.HasPrefix(name, "pb_") && now.Sub(session.created) >= time.Hour)
		cctvStreams.Unlock()
		if expired {
			_ = stopCCTVStream(id, name)
		}
		unlock()
	}
}

func StartCCTVStreamCleaner() {
	cctvCleanerOnce.Do(func() {
		go func() {
			for now := range time.NewTicker(30 * time.Second).C {
				sweepCCTVStreams(now)
			}
		}()
	})
}
