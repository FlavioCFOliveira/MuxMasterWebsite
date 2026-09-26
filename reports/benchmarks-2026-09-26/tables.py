#!/usr/bin/env python3
"""Generates the Markdown result tables of README.md from benchstat/csv/*.csv.

Every figure is copied from benchstat's output: medians, 95% confidence intervals
("± x%"), deltas and p-values. Nothing is recomputed except unit formatting.
Usage: python3 tables.py > tables.md
"""
import csv
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
CSV = os.path.join(HERE, "benchstat", "csv")


def sections(name):
    """Returns {unit: {column label: {row: {value, ci, delta, p}}}} for one CSV file."""
    rows = list(csv.reader(open(os.path.join(CSV, name))))
    out, i = {}, 0
    while i < len(rows):
        r = rows[i]
        if r and r[0] == "" and len(r) > 1 and i + 1 < len(rows) and rows[i + 1][1:2] and rows[i + 1][0] == "":
            labels, units = r, rows[i + 1]
            unit = units[1]
            cols, j = [], 1
            while j < len(units):
                if units[j] not in ("CI", "vs base", "P", ""):
                    lab = labels[j] if j < len(labels) and labels[j] else (cols[-1][0] if cols else "")
                    has_cmp = j + 2 < len(units) and units[j + 2] == "vs base"
                    cols.append((lab, j, has_cmp))
                    j += 4 if has_cmp else 2
                else:
                    j += 1
            table = {c[0]: {} for c in cols}
            i += 2
            while i < len(rows) and rows[i] and rows[i][0] not in ("",):
                row = rows[i]
                if row[0] != "geomean":
                    for lab, j, has_cmp in cols:
                        if j < len(row) and row[j] != "":
                            table[lab][row[0]] = {
                                "value": float(row[j]),
                                "ci": row[j + 1] if j + 1 < len(row) else "",
                                "delta": row[j + 2] if has_cmp and j + 2 < len(row) else "",
                                "p": row[j + 3] if has_cmp and j + 3 < len(row) else "",
                            }
                i += 1
            out.setdefault(unit, table)
        else:
            i += 1
    return out


def fmt_time(sec):
    ns = sec * 1e9
    for div, unit in ((1e6, "ms"), (1e3, "µs"), (1, "ns")):
        if ns >= div or unit == "ns":
            v = ns / div
            s = f"{v:#.4g}".rstrip(".") if v < 1000 else f"{v:.0f}"
            return f"{s} {unit}"


def fmt_num(v):
    return f"{v:.0f}" if abs(v - round(v)) < 1e-9 or v >= 1000 else f"{v:.4g}"


def allocs(v):
    return f"{fmt_num(v)} alloc" + ("" if v == 1 else "s")


def strip(name):
    return name[:-3] if name.endswith("-16") else name


def pval(p):
    return p.split(" ")[0].replace("p=", "") if p else ""


def delta(d):
    return "no significant difference" if d.strip() == "~" else d


def compare_table(csvname, a, b, names=None):
    s = sections(csvname)
    t, by, al = s["sec/op"], s["B/op"], s["allocs/op"]
    keys = [k for k in t[a] if k in t[b]]
    if names:
        keys = [k for k in keys if strip(k) in names]
    lines = [
        "| Benchmark | v1.1.0 time/op | v1.3.0 time/op | Change | p | v1.1.0 B/op → v1.3.0 | v1.1.0 allocs/op → v1.3.0 |",
        "|---|---|---|---|---|---|---|",
    ]
    for k in keys:
        x, y = t[a][k], t[b][k]
        bx, byy = by[a][k]["value"], by[b][k]["value"]
        ax, ay = al[a][k]["value"], al[b][k]["value"]
        bcell = f"identical ({fmt_num(bx)})" if bx == byy else f"{fmt_num(bx)} → {fmt_num(byy)} ({delta(by[b][k]['delta'])}, p={pval(by[b][k]['p'])})"
        acell = f"identical ({fmt_num(ax)})" if ax == ay else f"{fmt_num(ax)} → {fmt_num(ay)}"
        lines.append(
            f"| {strip(k)} | {fmt_time(x['value'])} ± {x['ci']} | {fmt_time(y['value'])} ± {y['ci']} "
            f"| {delta(y['delta'])} | {pval(y['p'])} | {bcell} | {acell} |"
        )
    return "\n".join(lines)


def absolute_table(csvname, col=None, names=None):
    s = sections(csvname)
    t, by, al = s["sec/op"], s["B/op"], s["allocs/op"]
    col = col or next(iter(t))
    keys = list(t[col])
    if names:
        keys = [k for k in keys if strip(k) in names]
    lines = ["| Benchmark | time/op | B/op | allocs/op |", "|---|---|---|---|"]
    for k in keys:
        v = t[col][k]
        lines.append(f"| {strip(k)} | {fmt_time(v['value'])} ± {v['ci']} | {fmt_num(by[col][k]['value'])} | {fmt_num(al[col][k]['value'])} |")
    return "\n".join(lines)


ROUTERS = ["MuxMaster", "MuxMasterPooled", "MuxMasterFast", "HTTProuter", "BunRouter", "Chi", "GorillaMux"]
LABEL = {
    "MuxMaster": "MuxMaster default",
    "MuxMasterPooled": "MuxMaster Pooled",
    "MuxMasterFast": "MuxMaster Fast",
    "HTTProuter": "httprouter",
    "BunRouter": "bunrouter (http.Handler)",
    "Chi": "chi v5",
    "GorillaMux": "gorilla/mux",
}
CATS = ["StaticRoute", "ParamRoute1", "ParamRoute2", "ParamRoute3", "WildcardRoute", "NotFound",
        "ParallelStaticRoute", "ParallelParamRoute"]


def competitor():
    base = {r: sections(f"competitor-v130-vs-{r}.csv") for r in ("MuxMaster", "MuxMasterPooled", "MuxMasterFast", "HTTProuter")}
    s = base["MuxMaster"]
    t, by, al = s["sec/op"], s["B/op"], s["allocs/op"]
    out = ["| Category | " + " | ".join(LABEL[r] for r in ROUTERS) + " |", "|---" * (len(ROUTERS) + 1) + "|"]
    for c in CATS:
        k = c + "-16"
        cells = []
        for r in ROUTERS:
            if k in t.get(r, {}):
                cells.append(f"{fmt_time(t[r][k]['value'])} ± {t[r][k]['ci']}<br>{fmt_num(by[r][k]['value'])} B, {allocs(al[r][k]['value'])}")
            else:
                cells.append("not measured")
        out.append(f"| {c} | " + " | ".join(cells) + " |")
    table = "\n".join(out)

    # Fastest per category, tested against the runner-up with benchstat's own comparison.
    fast = ["| Category | Fastest (median) | Runner-up (median) | Runner-up vs fastest | p | Fastest non-MuxMaster router |",
            "|---|---|---|---|---|---|"]
    for c in CATS:
        k = c + "-16"
        meds = sorted((t[r][k]["value"], r) for r in ROUTERS if k in t.get(r, {}))
        (v1, r1), (v2, r2) = meds[0], meds[1]
        cmp = base[r1]["sec/op"][r2][k]
        other = next((v, r) for v, r in meds if not r.startswith("MuxMaster"))
        oc = base[r1]["sec/op"][other[1]][k] if not r1 == other[1] else None
        other_cell = f"{LABEL[other[1]]} {fmt_time(other[0])}" + (
            f" ({delta(oc['delta'])} vs fastest, p={pval(oc['p'])})" if oc else " (fastest)")
        fast.append(f"| {c} | {LABEL[r1]} {fmt_time(v1)} | {LABEL[r2]} {fmt_time(v2)} | {delta(cmp['delta'])} | {pval(cmp['p'])} | {other_cell} |")

    # Every MuxMaster mode versus httprouter, per category (baseline: the MuxMaster mode).
    vs = ["| Category | MuxMaster mode | MuxMaster (median) | httprouter (median) | httprouter vs MuxMaster mode | p |",
          "|---|---|---|---|---|---|"]
    for c in CATS:
        k = c + "-16"
        for r in ROUTERS[:3]:
            if k not in t.get(r, {}):
                continue
            cmp = base[r]["sec/op"]["HTTProuter"][k]
            vs.append(f"| {c} | {LABEL[r].replace('MuxMaster ', '')} | {fmt_time(t[r][k]['value'])} | "
                      f"{fmt_time(t['HTTProuter'][k]['value'])} | {delta(cmp['delta'])} | {pval(cmp['p'])} |")
    return table, "\n".join(fast), "\n".join(vs)


def noise_floor():
    t = sections("root-v130-aa-noise-floor.csv")["sec/op"]
    a, b = list(t)[:2]
    sig = [(strip(k), t[b][k]["delta"], pval(t[b][k]["p"])) for k in t[b] if t[b][k]["delta"].strip() != "~"]
    return len(t[b]), sig


# Upstream "Measured changes since v1.1.0" items mapped to the benchmark that measures them.
# (item, csv, v1.1.0 column or None, v1.3.0 column, benchmark row)
PA, PAC = "perfaudit-v110-vs-v130.csv", ("pa-v110-clean.txt", "pa-v130-clean.txt")
RT, RTC = "root-v110-vs-v130.csv", ("root-v110.txt", "root-v130.txt")
CHANGES = [
    ("Route registration, 100 routes", RT, None, RTC[1], "RegisterRoutes/N=100"),
    ("Route registration, 1 000 routes", RT, None, RTC[1], "RegisterRoutes/N=1000"),
    ("Route registration, 5 000 routes", RT, None, RTC[1], "RegisterRoutes/N=5000"),
    ("`Mount`, static prefix (root suite)", RT, None, RTC[1], "Mount_Static"),
    ("`Mount`, parameterised prefix (root suite)", RT, None, RTC[1], "Mount_Param"),
    ("`Mount`, static prefix (waste-hunt harness)", "wastehunt-v130.csv", None, None, "Mount/current"),
    ("`CleanPath` middleware, dirty path", PA, *PAC, "Middleware_CleanPath_Dirty"),
    ("`CleanPath` middleware, dirty path (waste-hunt harness)", "wastehunt-v130.csv", None, None, "CleanPathChanged/current"),
    ("`StripSlashes` middleware, dirty path", PA, *PAC, "Middleware_StripSlashes_Dirty"),
    ("Trailing-slash redirect, no middleware", PA, *PAC, "RedirectTSL"),
    ("Trailing-slash redirect, 5-middleware chain", PA, None, PAC[1], "RedirectTSLWithMiddleware"),
    ("405 Method Not Allowed", PA, *PAC, "MethodNotAllowed"),
    ("Automatic `OPTIONS`", PA, *PAC, "OPTIONSAuto"),
    ("`Text` response helper", "wastehunt-v130.csv", None, None, "Text/current"),
    ("`ThrottlePerIP`, one client", PA, *PAC, "Middleware_ThrottlePerIP_Hit"),
    ("`ThrottleBacklog`, no wait", PA, *PAC, "Middleware_ThrottleBacklog_NoWait"),
    ("`RequestID`, generate", PA, *PAC, "Middleware_RequestID_Generate"),
    ("`RequestID`, propagate", PA, *PAC, "Middleware_RequestID_Propagate"),
    ("`Logger`", PA, *PAC, "Middleware_Logger"),
    ("`Compress`, small body", PA, *PAC, "Middleware_Compress_SmallBody"),
    ("`Compress`, large body", PA, *PAC, "Middleware_Compress_LargeBody"),
    ("`JWTAuth` HS256, valid token", PA, *PAC, "Middleware_JWTAuth_HS256_Hit"),
    ("`JWTAuth` HS256, invalid token", PA, *PAC, "Middleware_JWTAuth_HS256_Miss"),
    ("`RealIP`, `X-Forwarded-For`", PA, *PAC, "Middleware_RealIP_XFF"),
    ("`RealIP`, no header", PA, *PAC, "Middleware_RealIP_NoHeader"),
    ("`APIKey`, valid key", PA, *PAC, "Middleware_APIKey_Hit"),
    ("`APIKey`, invalid key", PA, *PAC, "Middleware_APIKey_Miss"),
    ("`BasicAuth`, valid credentials (security fix TSC-2026-0002)", PA, *PAC, "Middleware_BasicAuth_Hit"),
    ("`BasicAuth`, invalid credentials (security fix TSC-2026-0002)", PA, *PAC, "Middleware_BasicAuth_Miss"),
    ("`CORS`, no `Origin` header (security fix TM-2026-033)", PA, *PAC, "Middleware_CORS_NoOrigin"),
    ("`CORS`, allowed origin", PA, *PAC, "Middleware_CORS_AllowedOrigin"),
    ("`CORS`, preflight", PA, *PAC, "Middleware_CORS_Preflight"),
    ("`Recoverer`, no panic (security fix O-14)", PA, *PAC, "Middleware_Recoverer_NoPanic"),
]


def changes():
    out = ["| Item | Benchmark | v1.1.0 | v1.3.0 | Change | p |", "|---|---|---|---|---|---|"]
    for item, name, a, b, row in CHANGES:
        s = sections(name)
        t, al, by = s["sec/op"], s["allocs/op"], s["B/op"]
        b = b or next(iter(t))
        k = row + "-16"
        y = t[b][k]
        ycell = f"{fmt_time(y['value'])}, {fmt_num(by[b][k]['value'])} B, {allocs(al[b][k]['value'])}"
        if a is None:
            out.append(f"| {item} | `{row}` | not measurable (no benchmark at v1.1.0) | {ycell} | v1.3.0 only | — |")
        else:
            x = t[a][k]
            xcell = f"{fmt_time(x['value'])}, {fmt_num(by[a][k]['value'])} B, {allocs(al[a][k]['value'])}"
            out.append(f"| {item} | `{row}` | {xcell} | {ycell} | {delta(y['delta'])} | {pval(y['p'])} |")
    return "\n".join(out)


if __name__ == "__main__":
    w = sys.stdout.write
    n, sig = noise_floor()
    w(f"## Noise floor (A/A)\n\nrows={n} significant={sig}\n\n")
    w("## Root v1.1.0 vs v1.3.0\n\n" + compare_table("root-v110-vs-v130.csv", "root-v110.txt", "root-v130.txt") + "\n\n")
    new = sections("root-v110-vs-v130.csv")["sec/op"]
    only = {strip(k) for k in new["root-v130.txt"] if k not in new["root-v110.txt"]}
    w("## Root v1.3.0 only\n\n" + absolute_table("root-v110-vs-v130.csv", "root-v130.txt", names=only) + "\n\n")
    w("## Perf-audit v1.1.0 vs v1.3.0\n\n" + compare_table("perfaudit-v110-vs-v130.csv", "pa-v110-clean.txt", "pa-v130-clean.txt") + "\n\n")
    pa = sections(PA)["sec/op"]
    only = {strip(k) for k in pa[PAC[1]] if k not in pa[PAC[0]]}
    w("## Perf-audit v1.3.0 only\n\n" + absolute_table(PA, PAC[1], names=only) + "\n\n")
    w("## Middleware v1.3.0\n\n" + absolute_table("middleware-v130.csv") + "\n\n")
    w("## Waste-hunt v1.3.0\n\n" + absolute_table("wastehunt-v130.csv") + "\n\n")
    tab, fast, vs = competitor()
    w("## Changes\n\n" + changes() + "\n\n")
    w("## Competitor\n\n" + tab + "\n\n## Fastest\n\n" + fast + "\n\n## vs httprouter\n\n" + vs + "\n")
