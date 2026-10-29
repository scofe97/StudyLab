#!/usr/bin/env python3
"""
SVG 가독성 검증 — dev-standards 철칙 + 강화 기준(Δ≥90).
사용: python3 svg-contrast-check.py <file.svg>
나가는 코드: 0=통과, 1=경고(눈으로 재확인 필요)

검사 항목:
1. 어중간 어두운 박스 fill (배경색 #0d0d0d 외, 휘도 18~120)
2. rgba 박스 fill (반투명 박스 금지 — 배경 비침)
3. 텍스트 vs 그 텍스트가 놓인 배경(박스 또는 캔버스)의 휘도차
   - 일반 Δ<90 경고
   - 채도 높은 박스(파랑 #5b9bd5, 앰버 rgb(212,168,83)) 위 '진한 글자(L<60)'는
     숫자상 Δ가 커도 시각적으로 칙칙 → 별도 경고
4. 작은 폰트(<10px)면서 어두운 배경 위 중간색(L 100~180) → 묻힘 경고

⚠️ 이 스크립트는 *보조*다. 통과해도 반드시 rsvg-convert로 PNG 렌더해 눈으로 확인할 것.
   (휘도 숫자만으로는 못 잡는 케이스가 있음 — 2026-06-16 dispatch.svg 사고)
"""
import re, sys

def _rgb(c):
    c = c.strip()
    if c.startswith('rgb'):
        return [int(x) for x in re.findall(r'\d+', c)[:3]]
    c = c.lstrip('#')
    if len(c) == 3:
        c = ''.join(ch*2 for ch in c)
    return [int(c[0:2],16), int(c[2:4],16), int(c[4:6],16)]

def lum(c):
    r,g,b = _rgb(c)
    return 0.299*r + 0.587*g + 0.114*b

def sat(c):
    # 무채색(회색/흑백)이면 0에 가깝다. 앰버·초록 등 강조색은 크다.
    r,g,b = _rgb(c)
    return max(r,g,b) - min(r,g,b)

# 채도 높은 강조 박스 (이 위엔 진한 글자가 칙칙해짐)
SATURATED = {'#5b9bd5', 'rgb(212,168,83)', '#66bb6a'}

def main(path):
    s = open(path, encoding='utf-8').read()
    cls = {}
    for m in re.finditer(r'\.(\w+)\s*\{([^}]*)\}', s):
        fm = re.search(r'fill:\s*(#[0-9a-fA-F]{3,6}|rgba?\([^)]*\))', m.group(2))
        szm = re.search(r'font-size:\s*([\d.]+)', m.group(2))
        if fm:
            cls[m.group(1)] = (fm.group(1), float(szm.group(1)) if szm else 11)

    rects = []
    for m in re.finditer(r'<rect x="([\d.]+)" y="([\d.]+)" width="([\d.]+)" height="([\d.]+)"[^>]*?fill="([^"]+)"', s):
        x,y,w,h,f = float(m[1]),float(m[2]),float(m[3]),float(m[4]),m[5]
        if w >= 890 and h >= 590:  # canvas
            continue
        if f.strip() in ('none', 'transparent'):
            continue  # 윤곽선 전용 rect = 배경을 만들지 않는다
        if f.startswith('rgba'):
            a = float(re.findall(r'[\d.]+', f)[-1])
            if a < 0.5:
                continue  # transparent tint = background shows
            rects.append((x,y,w,h,'#0d0d0d'))
            continue
        rects.append((x,y,w,h,f))

    warns = []
    # 1+2: box fill checks
    for m in re.finditer(r'<rect[^>]*?fill="(#[0-9a-fA-F]{6}|rgb\([^)]*\))"', s):
        f = m.group(1)
        if f == '#0d0d0d':
            continue
        if 18 < lum(f) < 120:
            warns.append(f"[MID-DARK BOX] fill={f} L={lum(f):.0f} — 밝게 칠하거나 배경색(#0d0d0d)으로")
    if 'fill="rgba' in s and re.search(r'<rect[^>]*fill="rgba\([^)]*0\.[5-9]', s):
        warns.append("[RGBA BOX] 반투명 박스 fill — 불투명으로")

    def bg(px, py):
        best='#0d0d0d'; area=1e18
        for x,y,w,h,f in rects:
            if x<=px<=x+w and y<=py<=y+h and w*h<area:
                area=w*h; best=f
        return best

    # 3+4: text contrast
    for m in re.finditer(r'<text x="(-?[\d.]+)" y="(-?[\d.]+)"([^>]*)>([^<]*)</text>', s):
        px,py,attrs,txt = float(m[1]),float(m[2]),m[3],m[4]
        if not txt.strip():
            continue
        # ⚠️ CSS 우선순위: inline style="fill:" > <style>의 .class{fill:} > presentation fill="..."
        #    fill="#000" 을 줘도 .class 가 fill 을 지정하면 class 가 이긴다 (= fill= 무시됨).
        #    이 우선순위를 틀리면 "fill=로 고쳤는데 렌더는 클래스색 그대로"를 놓친다 (반복 사고 원인).
        st = re.search(r'style="fill:\s*(#[0-9a-fA-F]{3,6}|rgba?\([^)]*\))"', attrs)
        cm = re.search(r'class="(\w+)"', attrs)
        fa = re.search(r'\bfill="([^"]+)"', attrs)
        size = 11
        if st:                                   # 1순위: inline style
            fg = st.group(1)
        elif cm and cm.group(1) in cls and cls[cm.group(1)][0]:  # 2순위: class fill (fill= 보다 강함)
            fg, size = cls[cm.group(1)]
        elif fa:                                 # 3순위: presentation fill=
            fg = fa.group(1)
        else:
            fg = '#e8e4df'
        szm = re.search(r'font-size="([\d.]+)"', attrs)
        if szm: size = float(szm.group(1))
        b = bg(px, py)
        d = abs(lum(fg) - lum(b))
        t = txt.strip()[:30]
        # 강화 기준
        # 사용자 지침(SSOT): 밝은 박스 위 = 검정(#0d0d0d), 어두운 배경 위 = 흰색(#e8e4df).
        #   특정 강조(앰버 코드·+/- 마커 등)가 아니면 이 둘만 쓴다.
        # 따라서 "밝은 박스 위 검정"은 정상 — 경고하지 않는다.
        light_bg = lum(b) > 120
        # 중간 밝기 채도 박스(파랑 #5b9bd5 L142 등, L120~158)는 "밝은 박스"지만 검정 글자 대비가 약하다.
        # 더 밝은 앰버(L171)·초록(L152)은 검정이 또렷하나, 파랑 위 작은 검정/회색은 묻힌다.
        # → 중간 밝기 채도 박스 위는 흰색 권장. (Δ가 커도 시각적으로 칙칙 — 휘도가 못 잡는 케이스)
        # 파랑 #5b9bd5(L142)는 검정 글자가 묻히고 흰색이 또렷(2026-06-18 렌더 확인).
        # 초록 #66bb6a(L152)·앰버(L171)는 더 밝아 검정이 또렷 → 파랑만 예외.
        # 휘도 10 차이로는 파랑/초록 구분이 위험하므로 *파랑 색 자체*를 대상으로 한다.
        BLUE_MIDBOX = {'#5b9bd5'}
        midbox = b in BLUE_MIDBOX
        if midbox:
            # 파랑 위: 검정·회색은 묻힘 → 어두운 글자만 경고, 흰색은 정상.
            if lum(fg) < 130:
                warns.append(f"[DARK-ON-MIDBOX Δ={d:.0f}] '{t}' fg={fg}(L{lum(fg):.0f}) on {b}(파랑 L{lum(b):.0f}) — 파랑 위 어두운/회색 글자는 묻힘, 흰색(#e8e4df) 권장")
        elif light_bg and lum(fg) > 110:
            # 더 밝은 박스(앰버 L171·초록 L152 등) 위 밝은/중간 글자 — 묻힘. 검정이 정상.
            warns.append(f"[LIGHT-ON-LIGHT Δ={d:.0f}] '{t}' fg={fg}(L{lum(fg):.0f}) on {b}(L{lum(b):.0f}) — 밝은 박스 위는 검정(#0d0d0d)")
        elif not light_bg and lum(fg) < 70:
            # 어두운 배경 위 어두운 글자(검정·반투명검정 등) — 묻힘. 흰색(#e8e4df)으로.
            warns.append(f"[DARK-ON-DARK Δ={d:.0f}] '{t}' fg={fg}(L{lum(fg):.0f}) on {b}(L{lum(b):.0f}) — 어두운 배경 위는 흰색(#e8e4df)")
        elif not light_bg and sat(fg) < 40 and lum(fg) < 165:
            # 어두운 배경 위 무채색 회색(rgb(138,133,128) 등) — 흐림. 강조색(앰버·초록)이 아니면 흰색으로.
            # sat<40 = 채도 낮은 회색만(앰버 sat129·초록 sat85 등 강조색은 통과시킴).
            warns.append(f"[GREY-ON-DARK Δ={d:.0f}] '{t}' fg={fg}(L{lum(fg):.0f},sat{sat(fg):.0f}) on dark — 회색은 흐림, 흰색(#e8e4df)으로")
        elif d < 70:
            warns.append(f"[LOW Δ={d:.0f}] '{t}' fg={fg}(L{lum(fg):.0f}) bg={b}(L{lum(b):.0f})")

    if warns:
        print(f"⚠️  {len(warns)} 경고 — {path}")
        for w in warns:
            print("  " + w)
        print("\n→ 반드시 rsvg-convert로 PNG 렌더 후 눈으로 재확인할 것.")
        return 1
    print(f"✓ 휴리스틱 통과 — {path}")
    print("→ 그래도 rsvg-convert PNG 렌더 + 눈 확인 필수 (숫자가 못 잡는 케이스 존재).")
    return 0

if __name__ == '__main__':
    sys.exit(main(sys.argv[1]))
