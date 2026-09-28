import math
import numpy as np
from stats import trend, forecast, anomalies, changepoint, dow_shift, analyze


def geometric(n, start, rate):
    return [start * (1 + rate) ** i for i in range(n)]


def test_trend_detects_decline():
    y = geometric(12, 100, -0.03)
    t = trend(y)
    assert t is not None and t["significant"] and abs(t["slope_pct"] + 3) < 0.2


def test_trend_flat_is_not_significant():
    rng = np.random.default_rng(1)
    y = [100 * (1 + rng.normal(0, 0.05)) for _ in range(12)]
    t = trend(y)
    assert t is not None and not t["significant"]


def test_forecast_uses_factor_for_raw():
    y = geometric(12, 100, 0.0)
    f = forecast(y, [1.0, 0.5, 1.0, 1.0])
    assert f is not None and len(f["weeks"]) == 4
    assert f["weeks"][1]["raw"] == 2 * f["weeks"][1]["adj"]  # faktor 0,5 → minggu ramai
    assert f["weeks"][0]["lo"] <= f["weeks"][0]["raw"] <= f["weeks"][0]["hi"]


def test_anomaly_flags_single_spike():
    y = [100] * 12
    y[7] = 400
    a = anomalies(y)
    assert len(a) == 1 and a[0]["index"] == 7 and a[0]["z"] > 2.5


def test_changepoint_level_shift():
    y = [100] * 7 + [60] * 6
    y = [v * (1 + 0.01 * ((i % 3) - 1)) for i, v in enumerate(y)]
    cp = changepoint(y)
    assert cp is not None and cp["significant"] and cp["index"] == 7 and cp["shift_pct"] < -35


def test_dow_shift_weekend_drop():
    from datetime import date, timedelta
    daily = []
    start = date(2026, 8, 3)  # Senin
    for i in range(56):
        d = start + timedelta(days=i)
        base = 100 if d.weekday() >= 5 else 30
        if i >= 28 and d.weekday() >= 5:
            base = 60  # akhir pekan melemah 40% di 4 minggu terakhir
        daily.append({"d": d.isoformat(), "net": base, "kind": 0})
    s = dow_shift(daily)
    assert s is not None
    assert abs(s["weekend_pct"] + 40) < 1 and abs(s["weekday_pct"]) < 1
    assert s["worst_dow"] in (5, 6)


def test_dow_window_follows_block():
    from datetime import date, timedelta
    daily = []
    start = date(2026, 8, 3)
    for i in range(56):
        d = start + timedelta(days=i)
        base = 100 if d.weekday() >= 5 else 30
        if i >= 42:  # hanya 2 minggu terakhir yang melemah
            base = base // 2
        daily.append({"d": d.isoformat(), "net": base, "kind": 0})
    s14 = dow_shift(daily, recent_days=14)
    s28 = dow_shift(daily, recent_days=28)
    assert s14 is not None and abs(s14["weekend_pct"] + 50) < 1
    assert s28 is not None and s28["weekend_pct"] > -50  # jendela 4 mgg mencampur minggu normal


def test_analyze_end_to_end():
    weeks = [{"week_start": f"2026-0{6 + i // 4}-0{1 + (i % 4)}", "adj": v, "raw": v, "factor": 1}
             for i, v in enumerate(geometric(12, 100, -0.02))]
    out = analyze({"weeks": weeks, "daily": [], "future": [{"week_start": "x", "factor": 1}] * 4})
    assert out["trend"]["significant"] and out["forecast"]["weeks"][0]["week_start"] == "x"


def test_request_model_keeps_dow_days():
    # Pydantic membuang kolom yang tidak dideklarasikan; dow_days pernah hilang
    # diam-diam sehingga mode blok 2 minggu memakai jendela 4 minggu.
    from app import Series
    assert Series(dow_days=14).model_dump()["dow_days"] == 14
    assert Series().model_dump()["dow_days"] == 28
