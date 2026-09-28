"""
Statistik untuk Analisa Bisnis — dipanggil dari Go lewat app.py.

Semua fungsi murni (tanpa I/O) supaya bisa diuji sendiri. Deretnya adalah
penjualan mingguan SETARA KALENDER (sudah dikoreksi libur oleh Go), jadi
tren dan keanehan yang ditemukan di sini bukan libur — itu sudah dibuang.

Pilihan metodenya sengaja yang tahan pencilan: Theil–Sen untuk garis tren,
MAD untuk derau. Riwayat toko wisata pendek dan berayun; satu minggu meledak
tidak boleh menguasai tren maupun rentang perkiraan.
"""
from __future__ import annotations

import math
from dataclasses import dataclass, field
from typing import Optional

import numpy as np
from scipy import stats

MAD_K = 1.4826
Z80 = 1.2816          # setengah lebar rentang 80%
ANOMALY_Z = 2.5       # simpangan robust minimum untuk disebut minggu aneh
CHANGE_SCORE = 3.0    # skor minimum pergeseran level


def _mad_sigma(res: np.ndarray) -> float:
    if res.size == 0:
        return 0.0
    mad = float(np.median(np.abs(res - np.median(res)))) * MAD_K
    if mad > 0:
        return mad
    return float(np.std(res, ddof=1)) if res.size > 2 else 0.0


def _log_series(values):
    """Indeks & log dari nilai positif saja; nol/negatif = minggu kosong."""
    t, y = [], []
    for i, v in enumerate(values):
        if v is not None and v > 0:
            t.append(i)
            y.append(math.log(v))
    return np.array(t, dtype=float), np.array(y, dtype=float)


def trend(values, min_n: int = 6, recent_n: int = 8) -> Optional[dict]:
    """Kemiringan garis tren robust, % per minggu, dengan selang 95%."""
    t, y = _log_series(values)
    if t.size < min_n:
        return None

    def fit(tt, yy):
        slope, intercept, lo, hi = stats.theilslopes(yy, tt, alpha=0.95)
        return {
            "slope_pct": round((math.exp(slope) - 1) * 100, 2),
            "lo": round((math.exp(lo) - 1) * 100, 2),
            "hi": round((math.exp(hi) - 1) * 100, 2),
            "significant": bool(lo > 0 or hi < 0),
            "weeks": int(tt.size),
        }

    out = fit(t, y)
    if t.size >= recent_n and recent_n >= min_n:
        r = fit(t[-recent_n:], y[-recent_n:])
        out.update({
            "recent_slope_pct": r["slope_pct"],
            "recent_lo": r["lo"],
            "recent_hi": r["hi"],
            "recent_significant": r["significant"],
            "recent_weeks": r["weeks"],
        })
    return out


def _line(values, window: int = 13, min_n: int = 6):
    """Garis robust pada `window` titik terakhir: (slope, intercept, sigma, t, y)."""
    t, y = _log_series(values)
    if t.size < min_n:
        return None
    if t.size > window:
        t, y = t[-window:], y[-window:]
    slope, intercept, lo, hi = stats.theilslopes(y, t, alpha=0.95)
    if not (lo > 0 or hi < 0):
        # Tren tidak nyata → garis datar di nilai tengah 4 titik terakhir.
        slope = 0.0
        intercept = float(np.median(y[-4:]))
    res = y - (intercept + slope * t)
    return slope, intercept, _mad_sigma(res), t, y


def forecast(values, future_factors, horizon: int = 4) -> Optional[dict]:
    """Perkiraan `horizon` minggu ke depan, setara kalender & sebenarnya.

    future_factors[i] = faktor kalender minggu ke-i ke depan (bobot minggu
    biasa ÷ bobot minggu itu). Angka sebenarnya = setara ÷ faktor.
    """
    fit = _line(values)
    if fit is None:
        return None
    slope, intercept, sigma, t, y = fit
    n_total = len(values)
    weeks = []
    for h in range(1, horizon + 1):
        ti = (n_total - 1) + h
        mu = intercept + slope * ti
        s = sigma * math.sqrt(1 + 0.25 * (h - 1))
        adj = math.exp(mu)
        lo, hi = math.exp(mu - Z80 * s), math.exp(mu + Z80 * s)
        f = future_factors[h - 1] if h - 1 < len(future_factors) and future_factors[h - 1] else 1.0
        f = f if f > 0 else 1.0
        weeks.append({
            "h": h,
            "adj": round(adj),
            "raw": round(adj / f),
            "lo": round(lo / f),
            "hi": round(hi / f),
            "factor": round(f, 3),
        })
    return {
        "weeks": weeks,
        "total_raw": sum(w["raw"] for w in weeks),
        "total_lo": sum(w["lo"] for w in weeks),
        "total_hi": sum(w["hi"] for w in weeks),
        "sigma_log": round(sigma, 4),
        "basis": "tren" if slope != 0 else "datar",
    }


def anomalies(values, z_thresh: float = ANOMALY_Z, max_items: int = 3) -> list:
    """Minggu yang keluar dari rentang wajar garis tren (skala log)."""
    t, y = _log_series(values)
    if t.size < 6:
        return []
    slope, intercept, lo, hi = stats.theilslopes(y, t, alpha=0.95)
    if not (lo > 0 or hi < 0):
        slope, intercept = 0.0, float(np.median(y))
    fitted = intercept + slope * t
    res = y - fitted
    sigma = _mad_sigma(res)
    if sigma <= 0:
        return []
    out = []
    for i in range(t.size):
        z = res[i] / sigma
        if abs(z) >= z_thresh:
            out.append({
                "index": int(t[i]),
                "actual_adj": round(math.exp(y[i])),
                "lo": round(math.exp(fitted[i] - 2 * sigma)),
                "hi": round(math.exp(fitted[i] + 2 * sigma)),
                "z": round(float(z), 2),
            })
    return out[-max_items:]


def changepoint(values, min_seg: int = 3) -> Optional[dict]:
    """Satu titik di mana level (skala log) bergeser paling meyakinkan."""
    t, y = _log_series(values)
    n = y.size
    if n < 2 * min_seg + 1:
        return None
    best = None
    for k in range(min_seg, n - min_seg + 1):
        a, b = y[:k], y[k:]
        diff = float(b.mean() - a.mean())
        res = np.concatenate([a - a.mean(), b - b.mean()])
        sigma = _mad_sigma(res)
        if sigma <= 0:
            continue
        score = abs(diff) / (sigma * math.sqrt(1 / a.size + 1 / b.size))
        if best is None or score > best["score"]:
            best = {
                "index": int(t[k]),
                "shift_pct": round((math.exp(diff) - 1) * 100, 1),
                "before_mean": round(math.exp(a.mean())),
                "after_mean": round(math.exp(b.mean())),
                "score": round(score, 2),
                "significant": bool(score >= CHANGE_SCORE),
                "after_weeks": int(b.size),
            }
    return best


def dow_shift(daily, recent_days: int = 28) -> Optional[dict]:
    """Perubahan pola hari: 4 minggu terakhir vs 4 minggu sebelumnya, hari
    biasa saja (libur sudah dikeluarkan oleh Go lewat `kind`)."""
    rows = [r for r in daily if r.get("kind", 0) == 0 and r.get("net") is not None]
    rows.sort(key=lambda r: r["d"])
    if len(rows) < 2 * 7:
        return None
    # Dua jendela hari kalender, bukan hitungan baris: hari yang tutup pun
    # bagian dari polanya.
    last = rows[-1]["d"]
    from datetime import date, timedelta
    end = date.fromisoformat(last)
    cut = end - timedelta(days=recent_days - 1)
    start = cut - timedelta(days=recent_days)
    rec = {d: [] for d in range(7)}
    pri = {d: [] for d in range(7)}
    for r in rows:
        d = date.fromisoformat(r["d"])
        dow = d.weekday()  # 0 = Senin
        if d >= cut:
            rec[dow].append(r["net"])
        elif d >= start:
            pri[dow].append(r["net"])
    per = {}
    for d in range(7):
        if len(rec[d]) >= 2 and len(pri[d]) >= 2 and np.mean(pri[d]) > 0:
            per[d] = round((np.mean(rec[d]) / np.mean(pri[d]) - 1) * 100, 1)
    if len(per) < 4:
        return None

    def grp(days):
        a = [np.mean(rec[d]) for d in days if len(rec[d]) >= 2]
        b = [np.mean(pri[d]) for d in days if len(pri[d]) >= 2]
        if not a or not b or sum(b) <= 0:
            return None
        return round((sum(a) / sum(b) - 1) * 100, 1)

    weekend = grp([5, 6])
    weekday = grp([0, 1, 2, 3, 4])
    worst = min(per, key=lambda d: per[d])
    best = max(per, key=lambda d: per[d])
    return {
        "weekend_pct": weekend,
        "weekday_pct": weekday,
        "per_dow": per,
        "worst_dow": int(worst),
        "worst_pct": per[worst],
        "best_dow": int(best),
        "best_pct": per[best],
        "n_recent": int(sum(len(v) for v in rec.values())),
        "n_prior": int(sum(len(v) for v in pri.values())),
    }


def analyze(series: dict) -> dict:
    """series = {weeks: [{week_start, adj, raw, factor}], daily: [{d, net, kind}],
    future: [{week_start, factor}]}"""
    weeks = series.get("weeks") or []
    adj = [w.get("adj") for w in weeks]
    starts = [w.get("week_start") for w in weeks]
    future = series.get("future") or []
    out = {
        "trend": trend(adj),
        "forecast": forecast(adj, [f.get("factor") for f in future]),
        "anomalies": [],
        "changepoint": None,
        "dow": dow_shift(series.get("daily") or [], recent_days=int(series.get("dow_days") or 28)),
    }
    for a in anomalies(adj):
        a["week_start"] = starts[a["index"]] if a["index"] < len(starts) else None
        out["anomalies"].append(a)
    cp = changepoint(adj)
    if cp is not None:
        cp["week_start"] = starts[cp["index"]] if cp["index"] < len(starts) else None
        out["changepoint"] = cp
    if out["forecast"] is not None:
        for i, w in enumerate(out["forecast"]["weeks"]):
            w["week_start"] = future[i].get("week_start") if i < len(future) else None
    return out
