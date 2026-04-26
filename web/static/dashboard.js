const params   = new URLSearchParams(window.location.search);
const nickname = params.get('player') || '';

let allMatches = [];
let kdChart    = null;
let sortState  = { col: 'games', dir: 'desc' };

async function init() {
    if (!nickname) {
        showError('No player specified. <a href="/">Go back</a>.');
        return;
    }

    try {
        const [player, matches] = await Promise.all([
            fetchJSON(`/api/player/${encodeURIComponent(nickname)}`),
            fetchJSON(`/api/matches/${encodeURIComponent(nickname)}`),
        ]);

        allMatches = Array.isArray(matches) ? matches : [];

        renderHeader(player);
        renderKDChart(30);
        renderMapStats();
        renderPatterns();
        renderInsights();
        setupToggles();

        show('content');
        hide('loading');
    } catch (err) {
        showError(err.message);
    }
}

function renderHeader(player) {
    const flag   = countryFlag(player.country);
    const avatar = player.avatar_url
        ? `<img class="player-avatar" src="${player.avatar_url}" alt="" />`
        : `<div class="player-avatar-placeholder"></div>`;

    document.getElementById('player-header').innerHTML = `
    ${avatar}
    <div class="player-info">
      <div class="player-name">
        ${esc(player.nickname)}
        <span class="level-badge">Level ${player.level}</span>
      </div>
      <div class="player-meta">${flag} ${player.country.toUpperCase()}</div>
      <div class="elo-value">${player.current_elo} ELO</div>
    </div>
  `;
    document.title = `${player.nickname} — Faceit Analyzer`;
}

function renderKDChart(days) {
    const cutoff   = Date.now() / 1000 - days * 86400;
    const filtered = allMatches
        .filter(m => m.played_at >= cutoff)
        .slice()
        .sort((a, b) => a.played_at - b.played_at);

    const labels = filtered.map(m => formatDate(m.played_at));
    const kdData = filtered.map(m =>
        m.deaths > 0 ? parseFloat((m.kills / m.deaths).toFixed(2)) : m.kills);
    const rolling = rollingAvg(kdData, 5);

    const canvas = document.getElementById('kd-chart');
    const ctx    = canvas.getContext('2d');

    if (kdChart) kdChart.destroy();

    const grad = ctx.createLinearGradient(0, 0, 0, canvas.offsetHeight || 300);
    grad.addColorStop(0, 'rgba(255,85,0,0.35)');
    grad.addColorStop(1, 'rgba(255,85,0,0)');

    kdChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels,
            datasets: [
                {
                    label: 'K/D',
                    data: kdData,
                    borderColor: 'rgba(255,85,0,0.4)',
                    backgroundColor: 'transparent',
                    borderWidth: 1,
                    pointRadius: 2,
                    pointHoverRadius: 4,
                    tension: 0.3,
                },
                {
                    label: '5-match avg',
                    data: rolling,
                    borderColor: '#ff5500',
                    backgroundColor: grad,
                    borderWidth: 2,
                    pointRadius: 0,
                    pointHoverRadius: 4,
                    tension: 0.4,
                    fill: true,
                },
            ],
        },
        options: {
            responsive: true,
            interaction: { mode: 'index', intersect: false },
            plugins: {
                legend: { labels: { color: '#8888a8', font: { size: 12 } } },
                tooltip: {
                    backgroundColor: '#1a1a24',
                    borderColor: '#2a2a38',
                    borderWidth: 1,
                    titleColor: '#e8e8f0',
                    bodyColor: '#8888a8',
                },
            },
            scales: {
                x: {
                    ticks: { color: '#8888a8', maxTicksLimit: 10, font: { size: 11 } },
                    grid: { color: 'rgba(255,255,255,0.04)' },
                },
                y: {
                    ticks: { color: '#8888a8', font: { size: 11 } },
                    grid: { color: 'rgba(255,255,255,0.04)' },
                    min: 0,
                },
            },
        },
    });
}

function setupToggles() {
    document.querySelectorAll('.toggle-group button').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.toggle-group button')
                .forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            renderKDChart(parseInt(btn.dataset.days));
        });
    });
}

function computeMapStats(matches) {
    const maps = {};
    for (const m of matches) {
        const key = m.map || 'Unknown';
        if (!maps[key]) maps[key] = { wins: 0, total: 0, kills: 0, deaths: 0, headshots: 0 };
        const s = maps[key];
        s.total++;
        if (m.result === 'W') s.wins++;
        s.kills     += m.kills;
        s.deaths    += m.deaths;
        s.headshots += m.headshots;
    }
    return Object.entries(maps).map(([map, s]) => ({
        map,
        games:   s.total,
        winRate: s.total   ? s.wins / s.total * 100        : 0,
        kd:      s.deaths  ? s.kills / s.deaths             : s.kills,
        hsPct:   s.kills   ? s.headshots / s.kills * 100   : 0,
    }));
}

function renderMapStats() {
    const stats = computeMapStats(allMatches);
    sortAndRenderTable(stats);
}

function sortAndRenderTable(stats) {
    const { col, dir } = sortState;
    const sorted = [...stats].sort((a, b) => {
        const av = col === 'map' ? a[col] : parseFloat(a[col]);
        const bv = col === 'map' ? b[col] : parseFloat(b[col]);
        if (av < bv) return dir === 'asc' ? -1 :  1;
        if (av > bv) return dir === 'asc' ?  1 : -1;
        return 0;
    });

    const cols = [
        { key: 'map',     label: 'Map'   },
        { key: 'games',   label: 'Games' },
        { key: 'winRate', label: 'Win %' },
        { key: 'kd',      label: 'K/D'   },
        { key: 'hsPct',   label: 'HS %'  },
    ];

    const headers = cols.map(c => {
        let cls = '';
        if (c.key === col) cls = dir === 'asc' ? 'sort-asc' : 'sort-desc';
        return `<th class="${cls}" data-col="${c.key}">${c.label}</th>`;
    }).join('');

    const rows = sorted.map(s => {
        const winClass = s.winRate >= 50 ? 'win' : 'loss';
        return `
      <tr>
        <td>${formatMapName(s.map)}</td>
        <td>${s.games}</td>
        <td class="${winClass}">${s.winRate.toFixed(1)}%</td>
        <td>${s.kd.toFixed(2)}</td>
        <td>${s.hsPct.toFixed(1)}%</td>
      </tr>`;
    }).join('');

    const wrap = document.getElementById('map-stats-table');
    wrap.innerHTML = `
    <table>
      <thead><tr>${headers}</tr></thead>
      <tbody>${rows}</tbody>
    </table>`;

    wrap.querySelectorAll('th[data-col]').forEach(th => {
        th.addEventListener('click', () => {
            const newCol = th.dataset.col;
            if (sortState.col === newCol) {
                sortState.dir = sortState.dir === 'asc' ? 'desc' : 'asc';
            } else {
                sortState.col = newCol;
                sortState.dir = newCol === 'map' ? 'asc' : 'desc';
            }
            sortAndRenderTable(computeMapStats(allMatches));
        });
    });
}

function renderPatterns() {
    if (!allMatches.length) return;

    const total = allMatches.length;
    const wins  = allMatches.filter(m => m.result === 'W').length;

    const totalKills  = allMatches.reduce((s, m) => s + m.kills,     0);
    const totalDeaths = allMatches.reduce((s, m) => s + m.deaths,    0);
    const totalHS     = allMatches.reduce((s, m) => s + m.headshots, 0);

    const winRate = (wins / total * 100).toFixed(1);
    const avgKD   = totalDeaths > 0 ? (totalKills / totalDeaths).toFixed(2) : totalKills;
    const hsPct   = totalKills  > 0 ? (totalHS / totalKills * 100).toFixed(1) : '0.0';

    const last10    = allMatches.slice(0, 10);
    const l10Kills  = last10.reduce((s, m) => s + m.kills,  0);
    const l10Deaths = last10.reduce((s, m) => s + m.deaths, 0);
    const l10KD     = l10Deaths > 0 ? (l10Kills / l10Deaths).toFixed(2) : l10Kills;
    const trend     = parseFloat(l10KD) >= parseFloat(avgKD) ? '▲' : '▼';
    const trendCls  = parseFloat(l10KD) >= parseFloat(avgKD) ? 'win' : 'loss';

    document.getElementById('patterns-content').innerHTML = `
    <div class="stat-card">
      <div class="stat-value ${wins / total >= 0.5 ? 'win' : 'loss'}">${winRate}%</div>
      <div class="stat-label">Win Rate</div>
    </div>
    <div class="stat-card">
      <div class="stat-value">${avgKD}</div>
      <div class="stat-label">Avg K/D</div>
    </div>
    <div class="stat-card">
      <div class="stat-value">${hsPct}%</div>
      <div class="stat-label">HS Rate</div>
    </div>
    <div class="stat-card">
      <div class="stat-value ${trendCls}">${l10KD} ${trend}</div>
      <div class="stat-label">Last 10 K/D</div>
    </div>
  `;
}

function renderInsights() {
    const mapStats = computeMapStats(allMatches);
    const insights = generateInsights(mapStats, allMatches);
    const list     = document.getElementById('weak-spots-list');

    if (!insights.length) {
        list.innerHTML = `<li>No significant weak spots detected in the last ${allMatches.length} matches.</li>`;
        return;
    }
    list.innerHTML = insights.map(i => `<li>${i}</li>`).join('');
}

function generateInsights(mapStats, matches) {
    const insights = [];
    const qualified = mapStats.filter(m => m.games >= 5);

    if (qualified.length) {
        const worst = qualified.reduce((a, b) => a.winRate < b.winRate ? a : b);
        if (worst.winRate < 40) {
            insights.push(
                `You win only <strong>${worst.winRate.toFixed(0)}%</strong> of your games on ` +
                `<strong>${formatMapName(worst.map)}</strong> (${worst.games} games played).`
            );
        }
    }

    if (qualified.length) {
        const best = qualified.reduce((a, b) => a.winRate > b.winRate ? a : b);
        if (best.winRate >= 65) {
            insights.push(
                `<strong>${formatMapName(best.map)}</strong> is your strongest map — ` +
                `<strong>${best.winRate.toFixed(0)}%</strong> win rate over ${best.games} games.`
            );
        }
    }

    if (mapStats.length) {
        const mostPlayed = mapStats.reduce((a, b) => a.games > b.games ? a : b);
        if (mostPlayed.kd < 1.0) {
            insights.push(
                `Your K/D on <strong>${formatMapName(mostPlayed.map)}</strong> ` +
                `(your most-played map) is <strong>${mostPlayed.kd.toFixed(2)}</strong> — below 1.0.`
            );
        }
    }

    const totalKills = matches.reduce((s, m) => s + m.kills,     0);
    const totalHS    = matches.reduce((s, m) => s + m.headshots, 0);
    if (totalKills >= 50) {
        const hsPct = totalHS / totalKills * 100;
        if (hsPct < 25) {
            insights.push(
                `Low headshot rate: <strong>${hsPct.toFixed(0)}%</strong>. ` +
                `Aim training focused on head level could improve your K/D.`
            );
        } else if (hsPct >= 55) {
            insights.push(
                `High headshot rate of <strong>${hsPct.toFixed(0)}%</strong> — ` +
                `your aim is a real strength.`
            );
        }
    }

    const wins    = matches.filter(m => m.result === 'W').length;
    const winRate = wins / matches.length * 100;
    if (matches.length >= 20 && winRate < 40) {
        insights.push(
            `Overall win rate of <strong>${winRate.toFixed(0)}%</strong> over ` +
            `${matches.length} matches suggests a consistent performance issue — ` +
            `review your most-played maps for patterns.`
        );
    }

    const last10 = matches.slice(0, 10);
    if (last10.length >= 4) {
        const first  = last10[0].result;
        let   streak = 0;
        for (const m of last10) {
            if (m.result === first) streak++;
            else break;
        }
        if (streak >= 4) {
            insights.push(streak >= 4 && first === 'L'
                ? `You're on a <strong>${streak}-game losing streak</strong>. Consider taking a short break.`
                : `You're on a <strong>${streak}-game winning streak</strong> — keep the momentum going!`
            );
        }
    }

    if (matches.length >= 20) {
        const recent  = matches.slice(0, 10);
        const older   = matches.slice(10, 20);
        const kdOf    = arr => {
            const k = arr.reduce((s, m) => s + m.kills,  0);
            const d = arr.reduce((s, m) => s + m.deaths, 0);
            return d > 0 ? k / d : k;
        };
        const recentKD = kdOf(recent);
        const olderKD  = kdOf(older);
        if (recentKD < olderKD - 0.2) {
            insights.push(
                `Your K/D has dropped from <strong>${olderKD.toFixed(2)}</strong> to ` +
                `<strong>${recentKD.toFixed(2)}</strong> over the last 20 matches — you may be in a slump.`
            );
        } else if (recentKD > olderKD + 0.2) {
            insights.push(
                `Your K/D improved from <strong>${olderKD.toFixed(2)}</strong> to ` +
                `<strong>${recentKD.toFixed(2)}</strong> over the last 20 matches — good progress!`
            );
        }
    }

    return insights;
}

async function fetchJSON(url) {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${res.status} — player not found or API error`);
    return res.json();
}

function rollingAvg(data, window) {
    return data.map((_, i) => {
        const slice = data.slice(Math.max(0, i - window + 1), i + 1);
        return parseFloat((slice.reduce((s, v) => s + v, 0) / slice.length).toFixed(2));
    });
}

function formatDate(unix) {
    return new Date(unix * 1000).toLocaleDateString('en-GB', {
        day: '2-digit', month: 'short',
    });
}

function formatMapName(name) {
    return name.replace(/^(de_|cs_)/, '')
        .replace(/^\w/, c => c.toUpperCase());
}

function countryFlag(code) {
    if (!code || code.length !== 2) return '';
    return [...code.toUpperCase()]
        .map(c => String.fromCodePoint(0x1F1E6 - 65 + c.charCodeAt(0)))
        .join('');
}

function esc(str) {
    return str.replace(/[&<>"']/g, c =>
        ({ '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;' }[c]));
}

function show(id) { document.getElementById(id).classList.remove('hidden'); }
function hide(id) { document.getElementById(id).classList.add('hidden'); }

function showError(msg) {
    hide('loading');
    const el = document.getElementById('error-card');
    el.innerHTML = msg;
    el.classList.remove('hidden');
}

init();