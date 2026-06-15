package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/michael/device_grid/internal/store/repo"
	"github.com/michael/device_grid/internal/transport"
)

type NetCheckHandler struct {
	repos     repo.Repositories
	transport *transport.Manager
}

func NewNetCheckHandler(repos repo.Repositories, tm *transport.Manager) *NetCheckHandler {
	return &NetCheckHandler{repos: repos, transport: tm}
}

const streamingScript = `echo "===STREAMING==="
# Netflix
NF=$(curl -s --connect-timeout 5 --max-time 8 "https://www.netflix.com/title/81280792" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$NF" = "200" ]; then
  REGION=$(curl -s --connect-timeout 5 --max-time 8 "https://api.country.is" 2>/dev/null | grep -o '"country":"[A-Z]*"' | cut -d'"' -f4)
  echo "UNLOCK|Netflix|${REGION:-?}"
else
  NF2=$(curl -s --connect-timeout 5 --max-time 8 "https://www.netflix.com/title/70143836" -o /dev/null -w "%{http_code}" 2>/dev/null)
  if [ "$NF2" = "200" ]; then echo "PARTIAL|Netflix|仅自制剧"; else echo "LOCKED|Netflix|"; fi
fi
# Disney+
DS=$(curl -s --connect-timeout 5 --max-time 8 "https://disneyplus.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$DS" = "200" ] || [ "$DS" = "302" ]; then echo "UNLOCK|Disney+|"; else echo "LOCKED|Disney+|"; fi
# YouTube Premium
YT=$(curl -s --connect-timeout 5 --max-time 8 "https://www.youtube.com/premium" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$YT" = "200" ]; then echo "UNLOCK|YouTube Premium|"; else echo "LOCKED|YouTube Premium|"; fi
# TikTok
TT=$(curl -s --connect-timeout 5 --max-time 8 "https://www.tiktok.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$TT" = "200" ] || [ "$TT" = "302" ]; then echo "UNLOCK|TikTok|"; else echo "LOCKED|TikTok|"; fi
# Bilibili 港澳台
BILI=$(curl -s --connect-timeout 5 --max-time 8 "https://api.bilibili.com/pgc/player/web/playurl?avid=50762638&cid=100279344&qn=0&type=&otype=json&ep_id=268176&fourk=1&fnver=0&fnval=16&module=bangumi" 2>/dev/null | grep -o '"code":[0-9]*' | head -1)
if echo "$BILI" | grep -q '"code":0'; then echo "UNLOCK|Bilibili港澳台|"; else echo "LOCKED|Bilibili港澳台|"; fi
# 爱奇艺国际版
IQY=$(curl -s --connect-timeout 5 --max-time 8 "https://www.iq.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$IQY" = "200" ] || [ "$IQY" = "302" ]; then echo "UNLOCK|爱奇艺国际|"; else echo "LOCKED|爱奇艺国际|"; fi
# HBO Max
HBO=$(curl -s --connect-timeout 5 --max-time 8 "https://www.max.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$HBO" = "200" ] || [ "$HBO" = "301" ]; then echo "UNLOCK|HBO Max|"; else echo "LOCKED|HBO Max|"; fi
# Amazon Prime
PRIME=$(curl -s --connect-timeout 5 --max-time 8 "https://www.primevideo.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$PRIME" = "200" ] || [ "$PRIME" = "301" ]; then echo "UNLOCK|Amazon Prime|"; else echo "LOCKED|Amazon Prime|"; fi
# Paramount+
PARA=$(curl -s --connect-timeout 5 --max-time 8 "https://www.paramountplus.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$PARA" = "200" ] || [ "$PARA" = "301" ]; then echo "UNLOCK|Paramount+|"; else echo "LOCKED|Paramount+|"; fi
# Spotify
SP=$(curl -s --connect-timeout 5 --max-time 8 "https://www.spotify.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$SP" = "200" ] || [ "$SP" = "301" ]; then echo "AVAILABLE|Spotify|"; else echo "UNAVAILABLE|Spotify|"; fi
# TVB 香港无线电视
TVB=$(curl -s --connect-timeout 5 --max-time 8 "https://www.tvb.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$TVB" = "200" ] || [ "$TVB" = "301" ]; then echo "UNLOCK|TVB|"; else echo "LOCKED|TVB|"; fi
# AbemaTV 日本
ABEMA=$(curl -s --connect-timeout 5 --max-time 8 "https://api.abema.io/v1/ip/check?device=android" 2>/dev/null)
if echo "$ABEMA" | grep -q '"isoCountryCode":"JP"'; then echo "UNLOCK|AbemaTV|日本"; else echo "LOCKED|AbemaTV|"; fi
# DAZN 体育
DAZN=$(curl -s --connect-timeout 5 --max-time 8 "https://www.dazn.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$DAZN" = "200" ] || [ "$DAZN" = "301" ]; then echo "UNLOCK|DAZN|"; else echo "LOCKED|DAZN|"; fi
# Peacock (NBC)
PEACOCK=$(curl -s --connect-timeout 5 --max-time 8 "https://www.peacocktv.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$PEACOCK" = "200" ] || [ "$PEACOCK" = "301" ]; then echo "UNLOCK|Peacock|"; else echo "LOCKED|Peacock|"; fi
# Discovery+
DISC=$(curl -s --connect-timeout 5 --max-time 8 "https://www.discoveryplus.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$DISC" = "200" ] || [ "$DISC" = "301" ]; then echo "UNLOCK|Discovery+|"; else echo "LOCKED|Discovery+|"; fi
# Crunchyroll 动漫
CR=$(curl -s --connect-timeout 5 --max-time 8 "https://www.crunchyroll.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$CR" = "200" ] || [ "$CR" = "301" ]; then echo "UNLOCK|Crunchyroll|"; else echo "LOCKED|Crunchyroll|"; fi
# Now TV (香港)
NOWTV=$(curl -s --connect-timeout 5 --max-time 8 "https://www.nowtv.now.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$NOWTV" = "200" ] || [ "$NOWTV" = "302" ]; then echo "UNLOCK|Now TV|香港"; else echo "LOCKED|Now TV|"; fi
# Viu (亚洲)
VIU=$(curl -s --connect-timeout 5 --max-time 8 "https://www.viu.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$VIU" = "200" ] || [ "$VIU" = "301" ]; then echo "UNLOCK|Viu|"; else echo "LOCKED|Viu|"; fi
# KKBOX 音乐
KK=$(curl -s --connect-timeout 5 --max-time 8 "https://www.kkbox.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$KK" = "200" ] || [ "$KK" = "301" ]; then echo "AVAILABLE|KKBOX|"; else echo "UNAVAILABLE|KKBOX|"; fi
# Apple TV+
ATV=$(curl -s --connect-timeout 5 --max-time 8 "https://tv.apple.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$ATV" = "200" ]; then echo "UNLOCK|Apple TV+|"; else echo "LOCKED|Apple TV+|"; fi
# Canal+ (法国)
CANAL=$(curl -s --connect-timeout 5 --max-time 8 "https://www.canalplus.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$CANAL" = "200" ] || [ "$CANAL" = "301" ]; then echo "UNLOCK|Canal+|法国"; else echo "LOCKED|Canal+|"; fi
# Hotstar (印度/东南亚)
HS=$(curl -s --connect-timeout 5 --max-time 8 "https://www.hotstar.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$HS" = "200" ] || [ "$HS" = "301" ]; then echo "UNLOCK|Hotstar|"; else echo "LOCKED|Hotstar|"; fi
echo "===END==="
`

const aiScript = `echo "===AI==="
# ChatGPT
GPT=$(curl -s --connect-timeout 5 --max-time 8 "https://api.openai.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$GPT" = "200" ]; then echo "OK|ChatGPT|"; elif [ "$GPT" = "403" ]; then echo "LIMITED|ChatGPT|地区限制"; else echo "BLOCKED|ChatGPT|"; fi
# Claude AI
CL=$(curl -s --connect-timeout 5 --max-time 8 "https://claude.ai" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$CL" = "200" ] || [ "$CL" = "302" ]; then echo "OK|Claude AI|"; else echo "BLOCKED|Claude AI|"; fi
# Google Gemini
GM=$(curl -s --connect-timeout 5 --max-time 8 "https://gemini.google.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$GM" = "200" ] || [ "$GM" = "302" ] || [ "$GM" = "303" ]; then echo "OK|Google Gemini|"; else echo "BLOCKED|Google Gemini|"; fi
# Grok / xAI
GROK=$(curl -s --connect-timeout 5 --max-time 8 "https://x.ai" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$GROK" = "200" ] || [ "$GROK" = "302" ]; then echo "OK|Grok (xAI)|"; else echo "BLOCKED|Grok (xAI)|"; fi
# Mistral AI
MIST=$(curl -s --connect-timeout 5 --max-time 8 "https://chat.mistral.ai" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$MIST" = "200" ] || [ "$MIST" = "302" ]; then echo "OK|Mistral AI|"; else echo "BLOCKED|Mistral AI|"; fi
# Perplexity
PERP=$(curl -s --connect-timeout 5 --max-time 8 "https://www.perplexity.ai" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$PERP" = "200" ] || [ "$PERP" = "302" ]; then echo "OK|Perplexity|"; else echo "BLOCKED|Perplexity|"; fi
# Microsoft Copilot
COP=$(curl -s --connect-timeout 5 --max-time 8 "https://copilot.microsoft.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$COP" = "200" ] || [ "$COP" = "301" ]; then echo "OK|Microsoft Copilot|"; else echo "BLOCKED|Microsoft Copilot|"; fi
# DeepSeek
DS=$(curl -s --connect-timeout 5 --max-time 8 "https://platform.deepseek.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$DS" = "200" ]; then echo "OK|DeepSeek|"; else echo "BLOCKED|DeepSeek|"; fi
# GitHub Copilot
GH=$(curl -s --connect-timeout 5 --max-time 8 "https://api.githubcopilot.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$GH" = "200" ] || [ "$GH" = "401" ]; then echo "OK|GitHub Copilot|"; else echo "BLOCKED|GitHub Copilot|"; fi
# Hugging Face
HF=$(curl -s --connect-timeout 5 --max-time 8 "https://huggingface.co" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$HF" = "200" ] || [ "$HF" = "301" ]; then echo "OK|Hugging Face|"; else echo "BLOCKED|Hugging Face|"; fi
# Cohere
COH=$(curl -s --connect-timeout 5 --max-time 8 "https://dashboard.cohere.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$COH" = "200" ] || [ "$COH" = "301" ]; then echo "OK|Cohere|"; else echo "BLOCKED|Cohere|"; fi
# Together AI
TOG=$(curl -s --connect-timeout 5 --max-time 8 "https://api.together.xyz" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$TOG" = "200" ] || [ "$TOG" = "301" ]; then echo "OK|Together AI|"; else echo "BLOCKED|Together AI|"; fi
# Replicate
REP=$(curl -s --connect-timeout 5 --max-time 8 "https://replicate.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$REP" = "200" ] || [ "$REP" = "301" ]; then echo "OK|Replicate|"; else echo "BLOCKED|Replicate|"; fi
# Stability AI
STAB=$(curl -s --connect-timeout 5 --max-time 8 "https://platform.stability.ai" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$STAB" = "200" ] || [ "$STAB" = "301" ]; then echo "OK|Stability AI|"; else echo "BLOCKED|Stability AI|"; fi
# OpenRouter
OR=$(curl -s --connect-timeout 5 --max-time 8 "https://openrouter.ai" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$OR" = "200" ] || [ "$OR" = "301" ]; then echo "OK|OpenRouter|"; else echo "BLOCKED|OpenRouter|"; fi
# Groq
GROQ=$(curl -s --connect-timeout 5 --max-time 8 "https://groq.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$GROQ" = "200" ] || [ "$GROQ" = "301" ]; then echo "OK|Groq|"; else echo "BLOCKED|Groq|"; fi
# You.com
YOU=$(curl -s --connect-timeout 5 --max-time 8 "https://you.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$YOU" = "200" ]; then echo "OK|You.com|"; else echo "BLOCKED|You.com|"; fi
# Poe
POE=$(curl -s --connect-timeout 5 --max-time 8 "https://poe.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$POE" = "200" ]; then echo "OK|Poe|"; else echo "BLOCKED|Poe|"; fi
# Midjourney
MJ=$(curl -s --connect-timeout 5 --max-time 8 "https://www.midjourney.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$MJ" = "200" ] || [ "$MJ" = "301" ]; then echo "OK|Midjourney|"; else echo "BLOCKED|Midjourney|"; fi
# Runway
RW=$(curl -s --connect-timeout 5 --max-time 8 "https://runwayml.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$RW" = "200" ] || [ "$RW" = "301" ]; then echo "OK|Runway|"; else echo "BLOCKED|Runway|"; fi
# Suno AI
SUNO=$(curl -s --connect-timeout 5 --max-time 8 "https://suno.com" -o /dev/null -w "%{http_code}" 2>/dev/null)
if [ "$SUNO" = "200" ] || [ "$SUNO" = "301" ]; then echo "OK|Suno AI|"; else echo "BLOCKED|Suno AI|"; fi
echo "===END==="
`

// Connectivity test — uses ping with short timeout
const connectivityScript = `echo "===CONNECTIVITY==="
test_region() {
  local name="$1" host="$2"
  local output=$(ping -c 3 -W 2 "$host" 2>/dev/null)
  if [ $? -ne 0 ]; then echo "UNREACHABLE|$name|0|100"; return; fi
  local avg=$(echo "$output" | grep "rtt\|round-trip" | awk -F'/' '{print $5}' | awk '{printf "%.1f", $1}')
  local loss=$(echo "$output" | grep "packet loss" | grep -o '[0-9]*%' | tr -d '%')
  [ -z "$loss" ] && loss=0; [ -z "$avg" ] && avg=999
  echo "OK|$name|$avg|$loss"
}
test_region "中国大陆" "www.baidu.com"
test_region "中国香港" "www.google.com.hk"
test_region "日本" "www.yahoo.co.jp"
test_region "韩国" "www.naver.com"
test_region "新加坡" "www.gov.sg"
test_region "美国西部" "www.google.com"
test_region "欧洲" "www.bbc.co.uk"
test_region "Cloudflare" "1.1.1.1"
test_region "Google DNS" "8.8.8.8"
echo "===END==="
`

// Return route test — each target limited to 8s, total ~70s max
const returnRouteScript = `echo "===RETURNROUTE==="
test_route() {
  local name="$1" ip="$2"
  # Ping latency first (quick, 2 packets max 4s) — tagged for cleanup
  local ping_out=$(ping -c 2 -W 2 "$ip" 2>/dev/null)
  local avg="999"; local loss="100"
  if [ $? -eq 0 ]; then
    avg=$(echo "$ping_out" | grep "rtt\|round-trip" | awk -F'/' '{printf "%.1f", $5}')
    loss=$(echo "$ping_out" | grep "packet loss" | grep -o '[0-9]*%' | tr -d '%')
    [ -z "$loss" ] && loss=0; [ -z "$avg" ] && avg=999
  fi

  # Traceroute with hard 8s timeout per target
  local hops=""
  if command -v traceroute &>/dev/null; then
    hops=$(timeout 8 traceroute -n -w 1 -q 1 -m 10 "$ip" 2>/dev/null)
  elif command -v tracepath &>/dev/null; then
    hops=$(timeout 8 tracepath -m 10 "$ip" 2>/dev/null)
  fi

  # Detect line type from hops
  local linetype="Unknown"
  if [ -n "$hops" ]; then
    case "$hops" in
      *4809*|*cn2*|*gia*) linetype="CN2 GIA" ;;
      *)
        if echo "$hops" | grep -qiE '4134.*4809|4809'; then linetype="CN2 GT"; fi
        if echo "$hops" | grep -qiE '(^|[^0-9])4837([^0-9]|$)|9929'; then linetype="CU 9929/4837"; fi
        if echo "$hops" | grep -qiE '10099'; then linetype="CU AS10099"; fi
        if echo "$hops" | grep -qiE '58453|cmi'; then linetype="CM CMI"; fi
        if echo "$hops" | grep -qiE '(^|[^0-9])9808([^0-9]|$)'; then linetype="CM AS9808"; fi
        if echo "$hops" | grep -qiE '(^|[^0-9])4134([^0-9]|$)' && [ "$linetype" = "Unknown" ]; then linetype="CT 163"; fi
        if echo "$hops" | grep -qiE '3491|pccw' && [ "$linetype" = "Unknown" ]; then linetype="PCCW"; fi
        if echo "$hops" | grep -qiE '6453|tata' && [ "$linetype" = "Unknown" ]; then linetype="Tata"; fi
        if echo "$hops" | grep -qiE '1299|telia' && [ "$linetype" = "Unknown" ]; then linetype="Telia"; fi
        if echo "$hops" | grep -qiE '(^|[^0-9])174([^0-9]|$)|cogent' && [ "$linetype" = "Unknown" ]; then linetype="Cogent"; fi
        if echo "$hops" | grep -qiE '2914|ntt' && [ "$linetype" = "Unknown" ]; then linetype="NTT"; fi
        ;;
    esac
  fi

  echo "ROUTE|$name|$ip|$avg|$loss|$linetype"
  if [ -n "$hops" ]; then
    echo "$hops" | head -10 | while IFS= read -r line; do echo "HOP|$line"; done
  fi
  echo "---"
}

test_route "电信-北京" "219.141.140.10"
test_route "电信-上海" "202.96.209.133"
test_route "电信-广州" "202.96.128.86"
test_route "联通-北京" "123.125.81.6"
test_route "联通-上海" "210.22.70.3"
test_route "联通-广州" "210.21.196.6"
test_route "移动-北京" "211.136.17.107"
test_route "移动-上海" "211.136.150.66"
test_route "移动-广州" "120.196.165.24"
echo "===END==="
`

type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Region string `json:"region,omitempty"`
}

type ConnectivityResult struct {
	Region  string  `json:"region"`
	Latency float64 `json:"latency_ms"`
	Loss    int     `json:"loss_pct"`
	OK      bool    `json:"ok"`
}

type RouteResult struct {
	ISP      string   `json:"isp"`
	City     string   `json:"city"`
	IP       string   `json:"ip"`
	Latency  float64  `json:"latency_ms"`
	Loss     int      `json:"loss_pct"`
	LineType string   `json:"line_type"`
	Hops     []string `json:"hops"`
}

type CheckResponse struct {
	Results  interface{} `json:"results"`
	TestedAt time.Time   `json:"tested_at"`
}

func parseCheckResults(stdout string) []CheckResult {
	var results []CheckResult
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "===END") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 2 {
			continue
		}
		cr := CheckResult{Name: parts[1], Status: parts[0]}
		if len(parts) > 2 {
			cr.Region = parts[2]
		}
		results = append(results, cr)
	}
	return results
}

func parseConnectivityResults(stdout string) []ConnectivityResult {
	var results []ConnectivityResult
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "===END") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 3 {
			continue
		}
		cr := ConnectivityResult{Region: parts[1]}
		if parts[0] == "OK" && len(parts) >= 4 {
			fmt.Sscanf(parts[2], "%f", &cr.Latency)
			fmt.Sscanf(parts[3], "%d", &cr.Loss)
			cr.OK = true
		}
		results = append(results, cr)
	}
	return results
}

func parseRouteResults(stdout string) []RouteResult {
	var results []RouteResult
	var current *RouteResult
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "===END") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "ROUTE|") {
			parts := strings.SplitN(line, "|", 7)
			if len(parts) >= 6 {
				if current != nil {
					results = append(results, *current)
				}
				current = &RouteResult{}
				nameParts := strings.SplitN(parts[1], "-", 2)
				if len(nameParts) == 2 {
					current.ISP = nameParts[0]
					current.City = nameParts[1]
				} else {
					current.ISP = parts[1]
				}
				current.IP = parts[2]
				fmt.Sscanf(parts[3], "%f", &current.Latency)
				fmt.Sscanf(parts[4], "%d", &current.Loss)
				if len(parts) >= 7 {
					current.LineType = parts[5]
				}
			}
		} else if strings.HasPrefix(line, "HOP|") {
			if current != nil {
				current.Hops = append(current.Hops, strings.TrimPrefix(line, "HOP|"))
			}
		} else if line == "---" {
			if current != nil {
				results = append(results, *current)
				current = nil
			}
		}
	}
	if current != nil {
		results = append(results, *current)
	}
	return results
}

// wrapScript adds process cleanup before and after the actual script.
// A unique marker tag is used to avoid killing unrelated processes.
const scriptPrefix = `# Kill any leftover test processes from previous runs
pkill -f 'dg_netcheck_' 2>/dev/null; sleep 0.2
# Mark this shell for cleanup
export DG_CHECK_TAG=dg_netcheck_$$
# Trap EXIT to clean up children
trap 'pkill -f "$DG_CHECK_TAG" 2>/dev/null; kill $(jobs -p) 2>/dev/null' EXIT TERM INT
`

const scriptSuffix = `
# Final cleanup
pkill -f 'dg_netcheck_' 2>/dev/null
kill $(jobs -p) 2>/dev/null
`

// wrapScript prepends cleanup and appends final cleanup to a test script
func wrapScript(body string) string {
	return scriptPrefix + body + scriptSuffix
}

func (h *NetCheckHandler) StreamingCheck(c *gin.Context) {
	nodeID := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	result, err := h.transport.Exec(ctx, nodeID, wrapScript(streamingScript))
	if err != nil {
		Error(c, http.StatusBadGateway, "检测失败: "+err.Error())
		return
	}
	OK(c, CheckResponse{Results: parseCheckResults(result.Stdout), TestedAt: time.Now()})
}

func (h *NetCheckHandler) AICheck(c *gin.Context) {
	nodeID := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	result, err := h.transport.Exec(ctx, nodeID, wrapScript(aiScript))
	if err != nil {
		Error(c, http.StatusBadGateway, "检测失败: "+err.Error())
		return
	}
	OK(c, CheckResponse{Results: parseCheckResults(result.Stdout), TestedAt: time.Now()})
}

func (h *NetCheckHandler) ConnectivityTest(c *gin.Context) {
	nodeID := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	result, err := h.transport.Exec(ctx, nodeID, wrapScript(connectivityScript))
	if err != nil {
		Error(c, http.StatusBadGateway, "测试失败: "+err.Error())
		return
	}
	OK(c, CheckResponse{Results: parseConnectivityResults(result.Stdout), TestedAt: time.Now()})
}

func (h *NetCheckHandler) ReturnRouteTest(c *gin.Context) {
	nodeID := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 80*time.Second)
	defer cancel()
	result, err := h.transport.Exec(ctx, nodeID, wrapScript(returnRouteScript))
	if err != nil {
		Error(c, http.StatusBadGateway, "回程测试失败: "+err.Error())
		return
	}
	OK(c, CheckResponse{Results: parseRouteResults(result.Stdout), TestedAt: time.Now()})
}
