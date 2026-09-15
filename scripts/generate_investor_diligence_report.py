from __future__ import annotations

import json
import os
from collections import Counter
from pathlib import Path
from xml.sax.saxutils import escape

from pypdf import PdfReader, PdfWriter
from reportlab.graphics.charts.piecharts import Pie
from reportlab.graphics.shapes import Circle, Drawing, Rect, String
from reportlab.lib import colors
from reportlab.lib.colors import HexColor
from reportlab.lib.enums import TA_CENTER, TA_LEFT
from reportlab.lib.pagesizes import A4, landscape
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    BaseDocTemplate,
    Flowable,
    Frame,
    KeepTogether,
    PageBreak,
    PageTemplate,
    Paragraph,
    Spacer,
    Table,
    TableStyle,
)


ROOT = Path(__file__).resolve().parents[1]
EVIDENCE = ROOT / "docs" / "evidence" / "v4-testnet-validation"
OUTPUT = ROOT / "output" / "pdf" / "PoDL_Investor_Diligence_Report.pdf"
PAGE_W, PAGE_H = landscape(A4)
MARGIN_X, MARGIN_TOP, MARGIN_BOTTOM = 15 * mm, 14 * mm, 13 * mm
CONTENT_W = PAGE_W - 2 * MARGIN_X

NAVY = HexColor("#071827")
NAVY_2 = HexColor("#0D263A")
INK = HexColor("#102437")
MUTED = HexColor("#5C7081")
LINE = HexColor("#DCE7EC")
SOFT = HexColor("#F3F7F8")
CYAN = HexColor("#24D7C4")
TEAL = HexColor("#078B85")
BLUE = HexColor("#4E7CFF")
GOLD = HexColor("#E7A93F")
RED = HexColor("#D85C5C")
WHITE = colors.white


def read_json(name: str) -> dict:
    path = EVIDENCE / name
    return json.loads(path.read_text()) if path.exists() else {}


METRICS = read_json("metrics.json")
LOAD = read_json("load.json")
WALLET = read_json("wallet.json")
CONTRACTS = read_json("contracts.json")
BACKUP = read_json("backup-replay.json")
REPLAYS = [read_json(f"replay-{i}.json") for i in range(1, 5)]


def register_fonts() -> None:
    regular = "/System/Library/Fonts/Supplemental/Arial.ttf"
    bold = "/System/Library/Fonts/Supplemental/Arial Bold.ttf"
    if not os.path.exists(regular) or not os.path.exists(bold):
        raise FileNotFoundError("Arial fonts are required for the investor report")
    pdfmetrics.registerFont(TTFont("InvestorSans", regular))
    pdfmetrics.registerFont(TTFont("InvestorSansBold", bold))
    pdfmetrics.registerFontFamily("InvestorSans", normal="InvestorSans", bold="InvestorSansBold")


register_fonts()
BASE = getSampleStyleSheet()
BODY = ParagraphStyle("Body", parent=BASE["BodyText"], fontName="InvestorSans", fontSize=9.2, leading=13.4, textColor=INK, spaceAfter=5)
SMALL = ParagraphStyle("Small", parent=BODY, fontSize=7.5, leading=10.4, textColor=MUTED, spaceAfter=2)
TINY = ParagraphStyle("Tiny", parent=SMALL, fontSize=6.5, leading=8.6)
H1 = ParagraphStyle("H1", parent=BASE["Heading1"], fontName="InvestorSansBold", fontSize=24, leading=28, textColor=NAVY, spaceAfter=8)
H2 = ParagraphStyle("H2", parent=BASE["Heading2"], fontName="InvestorSansBold", fontSize=13, leading=16, textColor=NAVY_2, spaceBefore=4, spaceAfter=5)
H3 = ParagraphStyle("H3", parent=BASE["Heading3"], fontName="InvestorSansBold", fontSize=9.4, leading=12, textColor=TEAL, spaceAfter=3)
HEAD = ParagraphStyle("Head", parent=SMALL, fontName="InvestorSansBold", fontSize=7.0, leading=8.5, textColor=WHITE, alignment=TA_LEFT)
CELL = ParagraphStyle("Cell", parent=SMALL, fontSize=6.9, leading=9.0, textColor=INK)
CELL_CENTER = ParagraphStyle("CellCenter", parent=CELL, alignment=TA_CENTER)
HERO = ParagraphStyle("Hero", parent=H1, fontSize=32, leading=35, textColor=WHITE, spaceAfter=10)
HERO_SUB = ParagraphStyle("HeroSub", parent=BODY, fontSize=12, leading=17, textColor=HexColor("#B9CBD6"))


def P(text: str, style=BODY) -> Paragraph:
    return Paragraph(text, style)


def cells(row, style=CELL):
    return [P(escape(str(value)), style) for value in row]


def investor_table(headers, rows, widths, font_size=None, center_columns=()):
    data = [[P(escape(str(x)), HEAD) for x in headers]]
    for row in rows:
        rendered = []
        for index, value in enumerate(row):
            style = CELL_CENTER if index in center_columns else CELL
            if font_size:
                style = ParagraphStyle(f"cell-{font_size}-{index}", parent=style, fontSize=font_size, leading=font_size + 2)
            rendered.append(P(escape(str(value)), style))
        data.append(rendered)
    table = Table(data, colWidths=widths, repeatRows=1, hAlign="LEFT")
    table.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), NAVY_2),
        ("TEXTCOLOR", (0, 0), (-1, 0), WHITE),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("LEFTPADDING", (0, 0), (-1, -1), 6),
        ("RIGHTPADDING", (0, 0), (-1, -1), 6),
        ("TOPPADDING", (0, 0), (-1, -1), 5),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 5),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [WHITE, SOFT]),
        ("LINEBELOW", (0, 0), (-1, 0), 1.2, CYAN),
        ("GRID", (0, 1), (-1, -1), 0.25, LINE),
    ]))
    return table


def section(kicker: str, title: str, subtitle: str | None = None):
    parts = [P(kicker.upper(), ParagraphStyle("Kicker", parent=SMALL, fontName="InvestorSansBold", textColor=TEAL, fontSize=7.4, spaceAfter=4)), P(title, H1)]
    if subtitle:
        parts.append(P(subtitle, SMALL))
        parts.append(Spacer(1, 3 * mm))
    return parts


def callout(label: str, text: str, color=TEAL):
    t = Table([[P(label.upper(), ParagraphStyle("CalloutLabel", parent=SMALL, fontName="InvestorSansBold", textColor=color)), P(text, BODY)]], colWidths=[38 * mm, CONTENT_W - 38 * mm])
    t.setStyle(TableStyle([("BACKGROUND", (0, 0), (-1, -1), SOFT), ("BOX", (0, 0), (-1, -1), 0.7, color), ("VALIGN", (0, 0), (-1, -1), "MIDDLE"), ("LEFTPADDING", (0, 0), (-1, -1), 8), ("RIGHTPADDING", (0, 0), (-1, -1), 8), ("TOPPADDING", (0, 0), (-1, -1), 7), ("BOTTOMPADDING", (0, 0), (-1, -1), 7)]))
    return t


class ArchitectureFlow(Flowable):
    def __init__(self):
        super().__init__(); self.width = CONTENT_W; self.height = 94 * mm

    def draw(self):
        c = self.canv
        columns = [10, 72, 139, 207]
        widths = [51, 56, 57, 56]
        top = 66 * mm
        boxes = [
            ("Market inputs", "Traders\nLPs\nDAOs and token teams", CYAN),
            ("Liquidity intelligence", "3+ signed oracles\nDepth + demand\nVolatility + concentration", BLUE),
            ("PoDL execution", "DEX and router\nOpt-in strategy vault\nRemove - swap - add", GOLD),
            ("Settlement and evidence", "Signed BFT + QC\nState root + replay\nExplorer and reporting", TEAL),
        ]
        for i, (title, body, accent) in enumerate(boxes):
            x = columns[i] * mm; w = widths[i] * mm
            c.setFillColor(SOFT); c.roundRect(x, top, w, 27 * mm, 4 * mm, fill=1, stroke=0)
            c.setFillColor(accent); c.rect(x, top + 25 * mm, w, 2 * mm, fill=1, stroke=0)
            c.setFillColor(NAVY); c.setFont("InvestorSansBold", 9); c.drawString(x + 4 * mm, top + 19 * mm, title)
            c.setFillColor(MUTED); c.setFont("InvestorSans", 7.2)
            for j, line in enumerate(body.split("\n")): c.drawString(x + 4 * mm, top + (13 - j * 4.2) * mm, line)
            if i < 3:
                ax = x + w + 2 * mm; ay = top + 13 * mm
                c.setStrokeColor(TEAL); c.setLineWidth(1.2); c.line(ax, ay, ax + 7 * mm, ay)
                c.line(ax + 5 * mm, ay + 2 * mm, ax + 7 * mm, ay); c.line(ax + 5 * mm, ay - 2 * mm, ax + 7 * mm, ay)
        c.setFillColor(NAVY_2); c.roundRect(34 * mm, 22 * mm, 197 * mm, 25 * mm, 4 * mm, fill=1, stroke=0)
        c.setFillColor(WHITE); c.setFont("InvestorSansBold", 9); c.drawString(41 * mm, 39 * mm, "Security and capital constraints")
        c.setFont("InvestorSans", 7.2); c.setFillColor(HexColor("#C1D2DC"))
        c.drawString(41 * mm, 32 * mm, "Native slashable bond + capped liquidity credit")
        c.drawString(112 * mm, 32 * mm, "Circuit breakers + exposure limits + timelocks")
        c.drawString(190 * mm, 32 * mm, "FIFO exits + insurance reserve")
        c.setStrokeColor(TEAL); c.line(132 * mm, top, 132 * mm, 48 * mm)
        c.line(130 * mm, 50 * mm, 132 * mm, 48 * mm); c.line(134 * mm, 50 * mm, 132 * mm, 48 * mm)
        c.setFillColor(NAVY); c.setFont("InvestorSansBold", 10)
        c.drawCentredString(CONTENT_W / 2, 9 * mm, "Closed loop: observe -> score -> route or rebalance -> settle -> measure outcomes")


class RevenueModel(Flowable):
    def __init__(self):
        super().__init__(); self.width = CONTENT_W; self.height = 70 * mm

    def draw(self):
        c = self.canv
        pie = Pie(); pie.x = 0; pie.y = 5 * mm; pie.width = 58 * mm; pie.height = 58 * mm
        pie.data = [45, 35, 20]; pie.labels = ["LP 45%", "Insurance 35%", "Operations 20%"]
        for i, color in enumerate([TEAL, BLUE, GOLD]): pie.slices[i].fillColor = color
        pie.slices.strokeColor = WHITE; pie.slices.strokeWidth = 1
        pie.labels = None
        d = Drawing(65 * mm, 65 * mm); d.add(pie); d.drawOn(c, 0, 0)
        c.setFont("InvestorSansBold", 9); c.setFillColor(NAVY); c.drawString(64 * mm, 57 * mm, "Current effective allocation")
        legend = [(TEAL, "45% LP realized-yield bucket"), (BLUE, "35% insurance reserve"), (GOLD, "20% operations")]
        for i, (color, label) in enumerate(legend):
            y = (47 - i * 10) * mm; c.setFillColor(color); c.rect(64 * mm, y, 4 * mm, 4 * mm, fill=1, stroke=0)
            c.setFillColor(INK); c.setFont("InvestorSans", 7.5); c.drawString(71 * mm, y + 0.5 * mm, label)
        c.setFillColor(SOFT); c.roundRect(143 * mm, 5 * mm, 113 * mm, 57 * mm, 4 * mm, fill=1, stroke=0)
        c.setFillColor(NAVY); c.setFont("InvestorSansBold", 9); c.drawString(151 * mm, 53 * mm, "Revenue recognition rules")
        lines = [
            "Trading, routing and protocol fees",
            "B2B liquidity service invoices",
            "Vault management / performance fees",
            "Bridge and lending fees when launched",
            "Emissions excluded from revenue",
            "Slashing classified as security recovery",
        ]
        for i, line in enumerate(lines):
            y = (44 - i * 6.4) * mm; c.setFillColor(TEAL if i < 4 else RED); c.circle(153 * mm, y + 1 * mm, 1.2 * mm, fill=1, stroke=0)
            c.setFillColor(INK); c.setFont("InvestorSans", 7.2); c.drawString(158 * mm, y, line)


class PerformanceChart(Flowable):
    def __init__(self, intervals):
        super().__init__(); self.values = intervals; self.width = CONTENT_W; self.height = 59 * mm

    def draw(self):
        c = self.canv; counts = Counter(self.values); keys = sorted(counts); max_count = max(counts.values()) if counts else 1
        x0, y0, chart_h = 16 * mm, 14 * mm, 36 * mm
        chart_w = 150 * mm
        c.setStrokeColor(LINE); c.line(x0, y0, x0 + chart_w, y0)
        bar_w = 18 * mm; gap = (chart_w - len(keys) * bar_w) / max(1, len(keys) - 1)
        for i, key in enumerate(keys):
            x = x0 + i * (bar_w + gap); h = chart_h * counts[key] / max_count
            c.setFillColor(TEAL); c.roundRect(x, y0, bar_w, h, 1.5 * mm, fill=1, stroke=0)
            c.setFillColor(NAVY); c.setFont("InvestorSansBold", 7); c.drawCentredString(x + bar_w / 2, y0 + h + 3 * mm, str(counts[key]))
            c.setFont("InvestorSans", 7); c.drawCentredString(x + bar_w / 2, y0 - 5 * mm, f"{key}s")
        cards = [("2.15s", "Average"), ("2.00s", "Median"), ("4.00s", "P95"), ("6.00s", "Maximum")]
        for i, (value, label) in enumerate(cards):
            x = (180 + (i % 2) * 40) * mm; y = (35 - (i // 2) * 24) * mm
            c.setFillColor(SOFT); c.roundRect(x, y, 34 * mm, 18 * mm, 3 * mm, fill=1, stroke=0)
            c.setFillColor(NAVY); c.setFont("InvestorSansBold", 12); c.drawCentredString(x + 17 * mm, y + 10 * mm, value)
            c.setFillColor(MUTED); c.setFont("InvestorSans", 6.6); c.drawCentredString(x + 17 * mm, y + 4 * mm, label)


def page_background(canvas, doc):
    canvas.saveState()
    if doc.page == 1:
        canvas.setFillColor(NAVY); canvas.rect(0, 0, PAGE_W, PAGE_H, fill=1, stroke=0)
    else:
        canvas.setFillColor(WHITE); canvas.rect(0, 0, PAGE_W, PAGE_H, fill=1, stroke=0)
        canvas.setStrokeColor(LINE); canvas.line(MARGIN_X, 10 * mm, PAGE_W - MARGIN_X, 10 * mm)
        canvas.setFillColor(MUTED); canvas.setFont("InvestorSans", 6.5)
        canvas.drawString(MARGIN_X, 5.8 * mm, "PoDL | Investor diligence report")
        canvas.drawRightString(PAGE_W - MARGIN_X, 5.8 * mm, str(doc.page))
    canvas.restoreState()


def strip_dates(path: Path):
    reader = PdfReader(str(path)); writer = PdfWriter()
    for page in reader.pages: writer.add_page(page)
    metadata = {str(k): str(v) for k, v in (reader.metadata or {}).items() if v is not None and str(k) not in {"/CreationDate", "/ModDate"}}
    writer.add_metadata(metadata)
    tmp = path.with_suffix(".clean.pdf")
    with tmp.open("wb") as stream: writer.write(stream)
    tmp.replace(path)


story = []

# Cover
story += [Spacer(1, 22 * mm), P("PROOF OF DYNAMIC LIQUIDITY", ParagraphStyle("CoverKicker", parent=SMALL, fontName="InvestorSansBold", fontSize=9, textColor=CYAN, spaceAfter=7)), P("PoDL Investor\nDiligence Report", HERO), P("A liquidity coordination network that connects market intelligence, validator incentives, capital routing and measurable revenue.", HERO_SUB), Spacer(1, 15 * mm)]
cover = Table([
    [P("INVESTMENT THESIS", HEAD), P("CURRENT STAGE", HEAD), P("EVIDENCE AVAILABLE", HEAD)],
    [P("Turn fragmented liquidity data into safer routing, managed allocation and B2B liquidity services.", ParagraphStyle("CoverCell", parent=BODY, textColor=WHITE)), P("Pre-public-testnet engineering validation. No production revenue or independent decentralization is established.", ParagraphStyle("CoverCell2", parent=BODY, textColor=WHITE)), P("Four-process V4 network, signed wallet flow, 100-transfer workload, full replay/QC and restored-history verification.", ParagraphStyle("CoverCell3", parent=BODY, textColor=WHITE))],
], colWidths=[CONTENT_W / 3] * 3)
cover.setStyle(TableStyle([("BACKGROUND", (0, 0), (-1, 0), HexColor("#12344B")), ("BACKGROUND", (0, 1), (-1, -1), HexColor("#0B2234")), ("BOX", (0, 0), (-1, -1), 0.5, HexColor("#355063")), ("INNERGRID", (0, 0), (-1, -1), 0.5, HexColor("#355063")), ("VALIGN", (0, 0), (-1, -1), "TOP"), ("LEFTPADDING", (0, 0), (-1, -1), 10), ("RIGHTPADDING", (0, 0), (-1, -1), 10), ("TOPPADDING", (0, 0), (-1, -1), 8), ("BOTTOMPADDING", (0, 0), (-1, -1), 8)]))
story += [cover, Spacer(1, 13 * mm), P("Designed for investor review and independent market research", ParagraphStyle("CoverFoot", parent=SMALL, textColor=HexColor("#92A9B8"), fontSize=8)), P("Source repository: github.com/Zotish/PodlBlockchain", ParagraphStyle("CoverFoot2", parent=SMALL, textColor=HexColor("#92A9B8"), fontSize=8)), PageBreak()]

# Investment case
story += section("01 / Investment case", "What PoDL is building", "PoDL's opportunity is the integration of liquidity intelligence, capital execution and verifiable settlement inside one specialized network.")
thesis = Table([
    [P("Problem", H3), P("Solution", H3), P("Commercial entry", H3)],
    [P("Liquidity is fragmented across pools. Headline TVL can hide poor executable depth, concentration, volatility and wash activity."), P("Score useful liquidity, use it as bounded validator credit, route trades by net outcome, and move opted-in vault capital under explicit limits."), P("Start with liquidity analytics and treasury pilots; expand toward routing, vault and network fees after independent validation.")],
], colWidths=[CONTENT_W / 3] * 3)
thesis.setStyle(TableStyle([("BACKGROUND", (0, 0), (-1, -1), SOFT), ("BOX", (0, 0), (-1, -1), 0.5, LINE), ("INNERGRID", (0, 0), (-1, -1), 0.5, LINE), ("VALIGN", (0, 0), (-1, -1), "TOP"), ("LEFTPADDING", (0, 0), (-1, -1), 9), ("RIGHTPADDING", (0, 0), (-1, -1), 9), ("TOPPADDING", (0, 0), (-1, -1), 7), ("BOTTOMPADDING", (0, 0), (-1, -1), 7)]))
story += [thesis, Spacer(1, 5 * mm), ArchitectureFlow(), callout("Investor interpretation", "The investable thesis is a focused liquidity infrastructure company with a testnet protocol asset. Value must be proven through customer outcomes, repeat usage and collected revenue.", GOLD), PageBreak()]

# Stakeholder economics
story += section("02 / Stakeholder economics", "Benefits and disadvantages by participant", "Each benefit is a product hypothesis to validate. The trade-offs remain part of the investment case.")
story += [investor_table(
    ["Participant", "How they use PoDL", "Potential financial benefit", "Non-financial benefit", "Primary cost / disadvantage", "What proves value", "Current readiness"],
    [
        ["Trader", "Execute through the router / DEX", "Lower slippage or better net output", "One view of route quality and risk", "Gas, latency, MEV, route and bridge failure", "Matched-size net execution after all costs", "Routing code; live market proof pending"],
        ["Liquidity provider", "Deposit into approved pools or opt-in vault", "Trading fees plus variable realized distribution", "Managed allocation and proportional accounting", "Impermanent loss, asset decline, contract and withdrawal risk", "Net return versus HODL/static LP and successful exits", "Vault/AMM foundation; audit pending"],
        ["DAO / token project", "Buy analytics or liquidity service", "Lower execution cost and more productive treasury liquidity", "Risk reports and transparent policy", "Integration, support and strategy error", "Paid pilot, renewal and measured improvement", "Agreement backend; no paid pilot"],
        ["Developer / app", "Use APIs, SDK, contracts and routing", "Lower build cost or paid application demand", "Specialized liquidity primitives", "Small ecosystem and native runtime constraints", "External integration and retained usage", "Tooling exists; ecosystem unproven"],
        ["Validator", "Bond LQD, operate node, contribute qualified liquidity", "Fees and protocol rewards", "Role in network security and governance", "Infrastructure, slashing, key and token-price risk", "Independent profitable operation and uptime", "Four processes; zero independent operators"],
        ["Oracle operator", "Publish signed market observations", "Potential service fee", "Verifiable data contribution", "Data liability, uptime and signing-key risk", "Independent data sources and SLA history", "Three synthetic identities only"],
        ["Company investor", "Fund company milestones", "Equity/token upside only under actual terms", "Exposure to liquidity-infrastructure thesis", "Dilution, illiquidity, execution, regulatory and total-loss risk", "Traction, revenue, security review and defensibility", "Pre-revenue / early stage"],
        ["Token holder", "Pay gas, participate in protocol roles", "Utility demand may support value", "Access to network functions", "No automatic equity, revenue right or price guarantee", "Clear token terms and organic gas demand", "Utility implemented; demand unproven"],
    ], [25*mm, 35*mm, 39*mm, 35*mm, 46*mm, 47*mm, 39*mm], font_size=6.25)]
story += [Spacer(1, 4 * mm), callout("Value equation", "Net customer value = execution savings + additional realized fees - gas - slippage - keeper/service costs - impermanent loss and added risk.", TEAL), PageBreak()]

# Technical model
story += section("03 / Technical model", "How the system works", "The protocol separates market observation, capital decisions, consensus security and economic accounting.")
story += [investor_table(
    ["Layer", "Core mechanisms", "Purpose", "Control boundary", "Evidence in current release"],
    [
        ["Settlement", "Signed BFT, weighted QC, view change, deterministic replay, state root", "Finalize a shared state and stop without quorum", "ChainSpec and validator set", "Four local processes; replay/QC passed"],
        ["Security weighting", "Native bond plus capped verified-liquidity credit", "Link security to useful market activity while retaining slashable collateral", "Liquidity credit cannot replace native bond", "Implemented; independent economic calibration pending"],
        ["Liquidity intelligence", "Executable depth, organic demand, volatility, oracle confidence, LP concentration", "Distinguish useful liquidity from headline TVL", "Multi-source filters and circuit breaker", "Implemented; real-market validation pending"],
        ["Execution", "Amount-aware routing, multi-hop/split paths, AMMs, commit-reveal foundation", "Improve net execution and reduce manipulation exposure", "Min-out, deadlines, caps and approved assets", "Code and local tests; production liquidity absent"],
        ["Capital management", "Vault shares, NAV, FIFO exits, remove-swap-add rebalancing", "Move only opted-in capital toward approved opportunities", "Custody, withdrawal buffer, keeper and exposure limits", "Foundation present; complete public E2E/audit pending"],
        ["Economics", "Custody-backed revenue receipts, source ledger, waterfall, insurance, guarded buyback", "Separate business income from emissions", "Buyback disabled until policy conditions are met", "Accounting logic tested; no real revenue history"],
        ["Access", "Explorer, watch-only wallet, test signing, APIs and SDK", "Make state, accounts and metrics inspectable", "Raw private keys excluded from explorer", "Browser test transfer finalized"],
    ], [29*mm, 67*mm, 55*mm, 58*mm, 57*mm], font_size=6.5), PageBreak()]

# Differentiation
story += section("04 / Market position", "What is distinctive - and what already exists", "PoDL should be marketed as an integrated design. Liquidity-linked blockchain incentives already exist in the market.")
story += [investor_table(
    ["PoDL element", "Market precedent / overlap", "PoDL's proposed distinction", "Evidence today", "Diligence question"],
    [
        ["Liquidity-linked validator economics", "Berachain directs emissions and incentives through Proof-of-Liquidity reward vaults", "Consensus credit uses a mandatory native bond plus capped quality-scored liquidity credit", "Implemented in code and simulated", "Can the quality score resist capital-efficient manipulation?"],
        ["Dynamic liquidity movement", "Vaults, keepers and automated LP strategies exist throughout DeFi", "Routing signal and physical remove-swap-add movement are joined to chain policy and accounting", "Vault flow implemented; public seeded E2E pending", "Does it improve net outcomes after IL, gas and keeper cost?"],
        ["Liquidity quality score", "DEX analytics already measure depth, volume, volatility and concentration", "The score affects routing and bounded validator weight under one state transition", "Code and manipulation tests exist", "Is the formula stable across regimes and assets?"],
        ["Revenue integrity", "Protocols commonly publish fee and treasury dashboards", "Receipts separate realized business fees, emissions and slashing recovery", "Ledger/reconciliation implemented", "Can external financial accounts reconcile to on-chain receipts?"],
        ["Integrated liquidity L1", "Application-specific chains and custom L1s are established patterns", "PoDL packages consensus, DEX, vault, oracle, explorer and B2B service around one problem", "Broad engineering foundation", "Is a new L1 necessary versus deployment on an established chain?"],
    ], [46*mm, 60*mm, 70*mm, 46*mm, 44*mm], font_size=6.65), Spacer(1, 6 * mm),
    callout("Defensible claim", "PoDL combines useful-liquidity measurement, bounded security credit, governed capital movement and realized-revenue reporting. The combination still requires market proof and formal prior-art review.", GOLD), Spacer(1, 5 * mm),
    callout("Do not claim", "Globally first, impossible to hack, guaranteed principal, guaranteed yield, audited, decentralized or production-ready.", RED), PageBreak()]

# Comparison matrix
story += section("05 / Competitive landscape", "Seven-platform comparison", "Characteristics are drawn from official project documentation. Performance figures use different definitions and are not a throughput ranking.")
headers = ["Characteristic", "PoDL V4 candidate", "Bitcoin", "Ethereum", "Solana", "Sui", "Berachain", "Avalanche L1"]
rows = [
    ["Primary position", "Liquidity-specialized custom L1", "Monetary settlement", "General smart-contract platform", "High-throughput app platform", "Object-centric Move platform", "EVM liquidity-incentive L1", "Customizable sovereign L1 framework"],
    ["Consensus / security", "Signed BFT; native bond + capped liquidity credit", "Proof of Work", "Proof of Stake", "Stake-weighted PoH/PoS pipeline", "Mysticeti consensus; delegated PoS", "BERA stake + PoL reward allocation", "Avalanche consensus / configurable L1 validation"],
    ["Liquidity in base design", "Quality score affects routing and bounded validator weight", "No", "Mostly application layer", "Mostly application layer", "Mostly application layer", "Core reward-vault and incentive system", "Application or custom-VM choice"],
    ["Capital movement", "Opt-in vault remove-swap-add workflow", "No native DeFi", "Via applications", "Via applications", "Via applications", "Via applications / reward vaults", "Via EVM apps or custom VM"],
    ["Execution model", "Account/contract state; conflict-aware transfer lanes", "UTXO scripts", "EVM account state", "Account locks and parallel runtime", "Object-centric Move execution", "EVM-compatible app execution", "EVM or custom VM"],
    ["Smart-contract ecosystem", "Native Go plugin model; small ecosystem", "Constrained scripts", "Largest mature EVM ecosystem", "Rust / sBPF ecosystem", "Move packages", "EVM-identical ecosystem", "Subnet-EVM or custom VM"],
    ["Timing reference", "2.00s target; 2.15s average local sample", "~10 min target average", "12s slots", "~400ms current target; reduction planned", "Sub-second consensus evidence exists", "~2s reference / <3s average docs", "Configurable; example target block rate 2s"],
    ["TPS evidence used here", "31.64 API-to-observed-finality TPS for 100 signed transfers", "Not compared", "Not compared", "Not compared", "Not compared", "Not compared", "Not compared"],
    ["Operational maturity", "Local validation; no independent operators", "Global production network", "Global production network", "Global production network", "Global production network", "Global production network", "Production ecosystem and custom L1 tooling"],
    ["Main PoDL advantage to prove", "Closed liquidity-intelligence and execution loop", "Not comparable product focus", "Lower integration fragmentation", "Better liquidity-specific outcomes", "Better liquidity-specific outcomes", "Better quality-aware capital outcomes", "Need for specialized design over configurable L1"],
    ["Main PoDL disadvantage", "No mature liquidity, users, clients, audits or operator diversity", "Far lower trust history", "Far smaller ecosystem", "Unproven network capacity", "Unproven developer adoption", "Direct conceptual competitor already live", "Less tooling and interoperability"],
]
story += [investor_table(headers, rows, [27*mm] + [34.1*mm]*7, font_size=5.75), Spacer(1, 3 * mm), P("Sources: official documentation for Bitcoin, Ethereum, Solana, Sui, Berachain and Avalanche. Timing is a protocol or documentation reference, not transaction finality equivalence. PoDL measurements are from one Mac with four loopback validator processes and synthetic funds.", TINY), PageBreak()]

# Performance
sample = METRICS.get("block_sample", {})
intervals = sample.get("intervals_seconds", [])
story += section("06 / Measured performance", "Current V4 engineering evidence", "A bounded local integration measurement. It does not establish public WAN capacity, saturation TPS or production economics.")
story += [PerformanceChart(intervals), investor_table(
    ["Metric", "Measured result", "Test boundary", "Investor interpretation"],
    [
        ["Validator topology", "4 node processes / 1 Mac / 1 operator", "Loopback P2P", "Consensus integration evidence; no decentralization claim"],
        ["Block intervals", "99 intervals; average 2.15s; median 2s; P95 4s; max 6s", "Heights 34-133", "Close to target under short local workload; variability remains"],
        ["Mining snapshot", "76ms processing; 0ms schedule lag", "One endpoint snapshot", "Useful diagnostic only; no percentile or peak claim"],
        ["Transfer workload", "100/100 finalized in 3.16s; 31.64 TPS", "One sender, one recipient; API to observed inclusion", "Functional throughput sample; not maximum capacity"],
        ["Admission", "100 accepted / 0 failed; 39.81ms API batch admission", "Signatures generated before submission", "Shows batch admission path, not end-user latency distribution"],
        ["Resources", "266.28 MiB RSS and 3.2% CPU snapshot across four nodes", "Host snapshot after workload", "No peak, long-run or production sizing conclusion"],
        ["Fresh replay", "4/4 nodes passed; 114 signed non-reward transactions verified", "Genesis-to-tip execution and QC verification", "Supports deterministic history for this fresh V4 lab"],
        ["Restore verification", "26 files hash-matched; restored replay passed", "Stopped archive; no cloned validator signer started", "Data recovery evidence; service failover remains to test"],
    ], [42*mm, 56*mm, 81*mm, 87*mm], font_size=6.55), PageBreak()]

# Revenue
story += section("07 / Revenue model", "How PoDL can earn money", "The commercial model begins with recurring B2B services. Protocol fees become meaningful only after real usage and liquidity exist.")
story += [RevenueModel(), investor_table(
    ["Revenue line", "Payer", "Charging model", "Cost drivers", "Required evidence", "Status"],
    [
        ["Liquidity intelligence / reporting", "DAO, token treasury, market maker", "Fixed pilot then monthly subscription / SLA", "Data, infrastructure, analysis and support", "Paid pilot, renewal and measurable decision value", "Best near-term entry; no paying customer"],
        ["Liquidity-as-a-Service", "DAO or token project", "Setup + management fee; performance fee only if legal/reviewed", "Integration, capital, keepers, support and losses", "Net outcome after all costs and signed agreement", "Backend agreement/invoice model exists"],
        ["Routing / analytics API", "Wallet, DEX, appchain or trading app", "Usage tier or enterprise SLA", "Indexing, data, uptime and support", "External integration, conversion and retention", "API foundation; no sales"],
        ["Network and DEX fees", "Users and applications", "Per transaction / eligible trading activity", "Validators, infrastructure, incentives and security", "Organic volume, collected fees and cohort retention", "Mechanism exists; revenue history absent"],
        ["Strategy vault", "Professional LP or treasury", "Management fee + positive performance fee", "Custody, audit, keepers, insurance and legal operations", "Audited withdrawals and net results versus benchmark", "Later stage"],
        ["Bridge / lending / arbitrage", "Cross-chain user, borrower or protocol opportunity", "Activity-based fee / spread", "Security, liquidity, bad debt and monitoring", "Production integration and risk-adjusted realized receipts", "Optional later stage"],
    ], [42*mm, 40*mm, 53*mm, 48*mm, 55*mm, 28*mm], font_size=6.45), Spacer(1, 3 * mm), P("The policy defines 45% LP, 25% insurance, 20% operations and 10% guarded buyback. Because buyback is disabled by default, the effective current allocation is 45% LP, 35% insurance, 20% operations and 0% buyback. These are configurable protocol rules, not audited company profit distributions.", SMALL), PageBreak()]

# Business model
story += section("08 / Business model", "From service revenue to protocol usage", "The lowest-cost path for an independent founder is to sell a read-only product before operating customer capital.")
story += [investor_table(
    ["Stage", "Product", "Customer", "Sales motion", "Revenue proof", "Capital / risk level", "Expansion condition"],
    [
        ["1. Diagnostic", "Four-week liquidity health review", "Small DAO / token treasury", "Founder-led outreach and public-data assessment", "Collected fixed pilot fee", "Low: read-only, no custody", "Customer asks to repeat or renew"],
        ["2. Subscription", "Dashboard, alerts and recurring reports", "Treasury, wallet, DEX", "Monthly subscription or SLA", "MRR, gross margin, retention", "Low to medium: uptime/data obligations", "Three retained customers and support playbook"],
        ["3. Shadow optimization", "Rebalance simulation and routing API", "Treasury / market maker", "Paid trial against agreed baseline", "Measured net improvement after costs", "Medium: model and integration risk", "Repeatable outperformance and independent review"],
        ["4. Protocol-owned pilot", "Governed liquidity deployment", "Launch partner", "Capped agreement using protocol capital", "Realized fee receipts and safe exits", "High: capital and smart-contract risk", "Audit, insurance policy and incident drills"],
        ["5. Managed vault / network", "Automated allocation and public infrastructure", "Professional LPs, applications and validators", "Management, performance and activity fees", "AUM, volume, revenue and retention", "Very high: custody, legal and systemic risk", "Independent operators, audits and legal clearance"],
    ], [24*mm, 42*mm, 34*mm, 46*mm, 41*mm, 39*mm, 40*mm], font_size=6.4), Spacer(1, 5 * mm),
    callout("Unit economics", "Monthly contribution = collected customer fees - infrastructure - data - support and delivery cost. Protocol emissions and investment proceeds remain outside customer revenue.", TEAL), Spacer(1, 4 * mm),
    investor_table(["Leading KPI", "Why it matters", "First credible threshold"], [
        ["Paid pilots", "Tests willingness to pay", "Two scoped pilots; at least one paid"], ["Customer renewal", "Tests durable value", "One paid continuation, then three retained customers"], ["Net liquidity outcome", "Tests product advantage", "Matched baseline after gas, slippage, IL and service cost"], ["Gross margin", "Tests business quality", "Positive contribution after founder time is counted"], ["Protocol revenue", "Tests usage monetization", "Externally funded, custody-backed receipts separated from emissions"],
    ], [53*mm, 110*mm, 103*mm], font_size=6.7), PageBreak()]

# GTM
story += section("09 / Go-to-market", "Customer acquisition and adoption", "PoDL should win a narrow treasury workflow before asking the market to adopt a new Layer-1.")
story += [investor_table(
    ["Target", "Observed problem", "Initial offer", "Acquisition channel", "Activation event", "Retention signal", "Commercial outcome"],
    [
        ["Small token team", "Fragmented pools and weak liquidity reporting", "Free short assessment -> paid four-week review", "Founder outreach to 30 qualified teams", "Team reviews its own pool report", "Requests next weekly report", "Paid continuation / case study"],
        ["DAO treasury", "Unclear execution and rebalance cost", "Read-only shadow comparison", "Governance forums and treasury contributors", "Agrees baseline and risk limits", "Uses report in a second decision", "Subscription or scoped pilot"],
        ["Wallet / DEX", "Needs route quality and risk data", "API sandbox and benchmark", "Developer communities and direct integration", "Completes first quote integration", "Monthly active API calls", "Usage tier / SLA"],
        ["Developer tester", "Needs a reproducible chain and APIs", "Test missions and faucet", "Open-source and university communities", "Completes block, tx and wallet task", "Returns for a later test mission", "Contributor or integration lead"],
        ["Validator / oracle operator", "Needs clear operation and incentives", "Testnet runbook and synthetic assets", "Operator and university infrastructure groups", "Runs own key and host", "Completes recovery / incident drill", "Independent network capacity"],
    ], [27*mm, 42*mm, 46*mm, 42*mm, 38*mm, 37*mm, 34*mm], font_size=6.4), Spacer(1, 6 * mm),
    callout("Adoption targets", "25-100 real testers, 10-20 returning testers, four independently controlled validators, three independent oracle operators, two DAO/token-project pilots and one public benchmark report.", GOLD), Spacer(1, 5 * mm),
    callout("Founder strategy", "Use free infrastructure credits, public data and manual delivery. Spend scarce cash on independent review and customer delivery before token promotion or broad mainnet infrastructure.", TEAL), PageBreak()]

# Risk and milestones
story += section("10 / Investment risk", "What must be solved before real-fund mainnet", "The project has broad code coverage, but a production investment case depends on independent evidence and market adoption.")
story += [investor_table(
    ["Risk", "Why it matters", "Current control", "Missing evidence / action", "Investment consequence"],
    [
        ["Single-founder concentration", "Delivery, security and operations depend on one person", "Documentation and automated tooling", "Add maintainer, external reviewers and succession access", "Key-person discount and slower execution"],
        ["No independent decentralization", "One operator can control the tested network", "Four-process BFT and quorum-loss drill", "Separate hosts, keys, operators and jurisdictions", "No production security claim"],
        ["Liquidity/economic manipulation", "Attackers may manufacture volume or exploit oracle/weight formulas", "Caps, confidence filters, concentration and wash checks", "Live-market calibration and adversarial review", "Could invalidate core differentiation"],
        ["Custody and smart-contract loss", "Vault, bridge or lending defects can lose funds", "Limits, queues, journal, replay and insurance accounting", "Independent audits, bug bounty, withdrawal and incident drills", "Blocks real-fund product"],
        ["Product-market fit", "A technically complete chain may have no buyer", "Defined treasury beachhead and pilot model", "Paid pilot, renewal and measured net benefit", "Primary commercial risk"],
        ["Token / regulatory design", "Token rights, yield and cross-border sales may trigger obligations", "No guaranteed return or principal in product model", "Entity, jurisdiction, legal opinion and compliant terms", "Blocks fundraising/token distribution paths"],
        ["Performance and storage", "Short local tests do not predict WAN/long-run costs", "Incremental state, binary codec, replay and local measurements", "Multi-host saturation, long soak, disk-full and restore-service drill", "Infrastructure budget remains uncertain"],
        ["Competitive response", "Berachain and existing DeFi can copy or exceed features", "Integrated specialized workflow", "Customer data, reliable integrations, brand and operator network", "Feature-only moat is weak"],
    ], [37*mm, 56*mm, 58*mm, 70*mm, 45*mm], font_size=6.45), Spacer(1, 5 * mm),
    callout("Investment structure", "Fund milestone evidence: independent security review, multi-host public testnet, two customer pilots and measured retention. Avoid valuing the project from feature count or an unverified production TPS claim.", GOLD), PageBreak()]

# Diligence decision
story += section("11 / Investor decision", "What an investor can verify now", "The project is suitable for early technical and pre-seed diligence. It is not ready for claims based on production revenue, audited custody or decentralized mainnet operation.")
story += [investor_table(
    ["Diligence area", "Available now", "Still required", "Decision signal"],
    [
        ["Product", "Explorer, wallet test flow, chain, DEX/router/vault foundations", "Public user journey and complete seeded liquidity E2E", "Independent users complete core tasks"],
        ["Technology", "V4 genesis, signed BFT/QC, state replay, storage journal, pinned builtins", "Independent code audit, multi-client diversity and WAN operation", "Critical findings closed and release reproduced"],
        ["Performance", "2.15s average local block interval; 31.64 TPS bounded workload", "Comparable public multi-host latency/throughput and long-run resource profile", "Stable service under realistic traffic"],
        ["Market", "Defined DAO/token treasury problem and outreach plan", "Interviews, LOIs, paid pilots and renewals", "One customer pays again"],
        ["Economics", "Revenue-source ledger and insurance-first waterfall", "Real receipts, audited reconciliation and tested loss policy", "Revenue grows without emissions dependence"],
        ["Team", "Independent founder with broad implementation history", "Maintainer, security advisor and independent operators", "Reduced key-person and operational risk"],
        ["Legal", "Risk disclosures and restricted test-capital boundary", "Entity, jurisdiction and legal opinions", "Clear investment/token rights and operating permissions"],
    ], [42*mm, 77*mm, 92*mm, 55*mm], font_size=6.7), Spacer(1, 6 * mm),
    callout("Fundable proposition", "Finance the proof that quality-aware liquidity coordination creates repeatable customer value and recurring revenue. Release capital against evidence milestones.", TEAL), Spacer(1, 5 * mm),
    callout("Current conclusion", "PoDL is an ambitious integrated prototype with credible local engineering evidence and clear commercial hypotheses. Its value is still unproven until independent operators, security review, real liquidity and paying customers exist.", GOLD), PageBreak()]

# Sources
story += section("12 / Independent research", "Primary sources and verification map", "Investors can use these links to verify competing systems and compare PoDL's claims against official documentation.")
sources = [
    ("PoDL repository", "https://github.com/Zotish/PodlBlockchain", "Source implementation and project history"),
    ("Bitcoin block timing", "https://bitcoin.org/en/halving", "One block approximately every ten minutes on average"),
    ("Ethereum Proof of Stake", "https://ethereum.org/developers/docs/consensus-mechanisms/pos/", "12-second slots, validators and attestations"),
    ("Ethereum nodes and clients", "https://ethereum.org/developers/docs/nodes-and-clients/", "Independent execution and consensus client architecture"),
    ("Solana core transactions", "https://solana.com/docs/core/transactions", "Account-based transaction and instruction model"),
    ("Solana slot-time roadmap", "https://solana.com/uk/upgrades/reduced-slot-times", "Current 400ms reference and staged reduction plan"),
    ("Sui object model", "https://docs.sui.io/develop/sui-architecture/object-model", "Object-centric state and Move architecture"),
    ("Sui Mysticeti paper", "https://docs.sui.io/paper/mysticeti.pdf", "Consensus latency design and measurements"),
    ("Berachain Proof-of-Liquidity", "https://docs.berachain.com/general/proof-of-liquidity/overview", "Stake security, reward vaults, emissions and incentives"),
    ("Berachain performance FAQ", "https://docs.berachain.com/general/help/faqs", "Block-time and single-slot-finality references"),
    ("Avalanche virtual machines", "https://docs.avax.network/docs/primary-network/virtual-machines", "EVM and custom VM architecture"),
    ("Avalanche L1 model", "https://docs.avax.network/docs/avalanche-l1s", "Custom sovereign L1 rationale and trade-offs"),
]
source_rows = []
for name, url, use in sources:
    source_rows.append([name, f'<link href="{url}" color="#078B85">{url}</link>', use])
data = [[P("Source", HEAD), P("Official URL", HEAD), P("What it verifies", HEAD)]] + [[P(escape(name), CELL), P(url, CELL), P(escape(use), CELL)] for name, url, use in source_rows]
source_table = Table(data, colWidths=[52*mm, 132*mm, 82*mm], repeatRows=1)
source_table.setStyle(TableStyle([("BACKGROUND", (0, 0), (-1, 0), NAVY_2), ("VALIGN", (0, 0), (-1, -1), "TOP"), ("ROWBACKGROUNDS", (0, 1), (-1, -1), [WHITE, SOFT]), ("GRID", (0, 1), (-1, -1), 0.25, LINE), ("LEFTPADDING", (0, 0), (-1, -1), 6), ("RIGHTPADDING", (0, 0), (-1, -1), 6), ("TOPPADDING", (0, 0), (-1, -1), 4), ("BOTTOMPADDING", (0, 0), (-1, -1), 4), ("LINEBELOW", (0, 0), (-1, 0), 1.2, CYAN)]))
story += [source_table, Spacer(1, 4 * mm), P("PoDL evidence used in this report: docs/evidence/v4-testnet-validation, docs/throughput-v3-benchmark.md, docs/FOUNDER_BUSINESS_AND_LAUNCH_PLAN.md, BlockchainComponent/product_model.go and BlockchainComponent/economic_policy.go. The older 133,421 TPS pipeline result is excluded from the competitive headline because signature admission occurred outside its timed stage and it used one validator.", TINY), Spacer(1, 3 * mm), P("This report contains no investment-return forecast, token valuation, guaranteed yield or claim that PoDL is globally unique. Investors should commission their own technical, legal, financial and prior-art diligence.", TINY)]


OUTPUT.parent.mkdir(parents=True, exist_ok=True)
doc = BaseDocTemplate(str(OUTPUT), pagesize=(PAGE_W, PAGE_H), rightMargin=MARGIN_X, leftMargin=MARGIN_X, topMargin=MARGIN_TOP, bottomMargin=MARGIN_BOTTOM, title="PoDL Investor Diligence Report", author="PoDL Founding Team", subject="Investor-facing business, technical, market and performance assessment")
frame = Frame(MARGIN_X, MARGIN_BOTTOM, CONTENT_W, PAGE_H - MARGIN_TOP - MARGIN_BOTTOM, id="main", leftPadding=0, rightPadding=0, topPadding=0, bottomPadding=0)
doc.addPageTemplates([PageTemplate(id="investor", frames=[frame], onPage=page_background)])
doc.build(story)
strip_dates(OUTPUT)
print(OUTPUT)
