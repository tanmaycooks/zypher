// webscrapper — frontend interactions
// typewriter, particles, cursor glow, pill toggle, search + url modes

const $ = id => document.getElementById(id);
let mode = 'search';
let busy = false;

// ============================================
// INTERACTIVE CONSTELLATION NETWORK
// Nodes drift, connect by proximity, react to
// mouse — lines glow, pulses travel edges,
// cursor pushes nodes away.
// ============================================
(function initNetwork() {
    const canvas = $('particle-canvas');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    let w, h;
    const CONNECT_DIST = 150;
    const MOUSE_DIST = 200;
    const MOUSE_PUSH = 80;
    const NODE_COUNT = 65;

    let mx = -9999, my = -9999;
    let mouseActive = false;

    function resize() {
        w = canvas.width = window.innerWidth;
        h = canvas.height = window.innerHeight;
    }
    resize();
    window.addEventListener('resize', resize);

    document.addEventListener('mousemove', e => {
        mx = e.clientX; my = e.clientY;
        mouseActive = true;
    });
    document.addEventListener('mouseleave', () => { mouseActive = false; });

    // Nodes
    const nodes = [];
    for (let i = 0; i < NODE_COUNT; i++) {
        nodes.push({
            x: Math.random() * w,
            y: Math.random() * h,
            vx: (Math.random() - 0.5) * 0.4,
            vy: (Math.random() - 0.5) * 0.4,
            r: Math.random() * 2 + 1,
            baseAlpha: Math.random() * 0.3 + 0.15,
            alpha: 0.2,
            hue: Math.random() > 0.7 ? 20 : 15, // warm orange tones
        });
    }

    // Pulses — glowing dots that travel along edges
    const pulses = [];
    let pulseTimer = 0;

    function spawnPulse() {
        if (nodes.length < 2) return;
        const a = Math.floor(Math.random() * nodes.length);
        let b = a;
        let bestDist = Infinity;
        // find closest neighbor
        for (let i = 0; i < nodes.length; i++) {
            if (i === a) continue;
            const dx = nodes[i].x - nodes[a].x;
            const dy = nodes[i].y - nodes[a].y;
            const d = Math.sqrt(dx * dx + dy * dy);
            if (d < CONNECT_DIST && d < bestDist) {
                bestDist = d;
                b = i;
            }
        }
        if (b !== a) {
            pulses.push({ from: a, to: b, t: 0, speed: 0.01 + Math.random() * 0.02 });
        }
    }

    function draw() {
        ctx.clearRect(0, 0, w, h);

        // Spawn pulses periodically
        pulseTimer++;
        if (pulseTimer % 40 === 0) spawnPulse();

        // Update nodes
        for (const n of nodes) {
            // Drift
            n.x += n.vx;
            n.y += n.vy;

            // Bounce off edges softly
            if (n.x < 0) { n.x = 0; n.vx *= -1; }
            if (n.x > w) { n.x = w; n.vx *= -1; }
            if (n.y < 0) { n.y = 0; n.vy *= -1; }
            if (n.y > h) { n.y = h; n.vy *= -1; }

            // Mouse push
            if (mouseActive) {
                const dx = n.x - mx;
                const dy = n.y - my;
                const dist = Math.sqrt(dx * dx + dy * dy);
                if (dist < MOUSE_PUSH && dist > 0) {
                    const force = (MOUSE_PUSH - dist) / MOUSE_PUSH * 0.8;
                    n.vx += (dx / dist) * force;
                    n.vy += (dy / dist) * force;
                }
                // Glow near cursor
                n.alpha = dist < MOUSE_DIST
                    ? n.baseAlpha + (1 - dist / MOUSE_DIST) * 0.5
                    : n.baseAlpha;
            } else {
                n.alpha += (n.baseAlpha - n.alpha) * 0.05;
            }

            // Dampen velocity
            n.vx *= 0.995;
            n.vy *= 0.995;
        }

        // Draw connections
        for (let i = 0; i < nodes.length; i++) {
            for (let j = i + 1; j < nodes.length; j++) {
                const a = nodes[i], b = nodes[j];
                const dx = a.x - b.x;
                const dy = a.y - b.y;
                const dist = Math.sqrt(dx * dx + dy * dy);
                if (dist < CONNECT_DIST) {
                    const opacity = (1 - dist / CONNECT_DIST) * 0.15;
                    ctx.beginPath();
                    ctx.moveTo(a.x, a.y);
                    ctx.lineTo(b.x, b.y);
                    ctx.strokeStyle = `rgba(232,115,74,${opacity})`;
                    ctx.lineWidth = 0.6;
                    ctx.stroke();
                }
            }
        }

        // Draw mouse connections
        if (mouseActive) {
            for (const n of nodes) {
                const dx = n.x - mx;
                const dy = n.y - my;
                const dist = Math.sqrt(dx * dx + dy * dy);
                if (dist < MOUSE_DIST) {
                    const opacity = (1 - dist / MOUSE_DIST) * 0.25;
                    ctx.beginPath();
                    ctx.moveTo(mx, my);
                    ctx.lineTo(n.x, n.y);
                    ctx.strokeStyle = `rgba(240,144,112,${opacity})`;
                    ctx.lineWidth = 0.8;
                    ctx.stroke();
                }
            }
            // Mouse node glow
            const grad = ctx.createRadialGradient(mx, my, 0, mx, my, 60);
            grad.addColorStop(0, 'rgba(232,115,74,0.08)');
            grad.addColorStop(1, 'rgba(232,115,74,0)');
            ctx.beginPath();
            ctx.arc(mx, my, 60, 0, Math.PI * 2);
            ctx.fillStyle = grad;
            ctx.fill();
        }

        // Draw pulses
        for (let i = pulses.length - 1; i >= 0; i--) {
            const p = pulses[i];
            p.t += p.speed;
            if (p.t > 1) { pulses.splice(i, 1); continue; }
            const a = nodes[p.from], b = nodes[p.to];
            const px = a.x + (b.x - a.x) * p.t;
            const py = a.y + (b.y - a.y) * p.t;
            const glow = ctx.createRadialGradient(px, py, 0, px, py, 6);
            glow.addColorStop(0, `rgba(240,144,112,${0.7 * (1 - p.t)})`);
            glow.addColorStop(1, 'rgba(240,144,112,0)');
            ctx.beginPath();
            ctx.arc(px, py, 6, 0, Math.PI * 2);
            ctx.fillStyle = glow;
            ctx.fill();
        }

        // Draw nodes
        for (const n of nodes) {
            // Outer glow
            const glow = ctx.createRadialGradient(n.x, n.y, 0, n.x, n.y, n.r * 4);
            glow.addColorStop(0, `hsla(${n.hue},80%,60%,${n.alpha * 0.3})`);
            glow.addColorStop(1, `hsla(${n.hue},80%,60%,0)`);
            ctx.beginPath();
            ctx.arc(n.x, n.y, n.r * 4, 0, Math.PI * 2);
            ctx.fillStyle = glow;
            ctx.fill();
            // Core dot
            ctx.beginPath();
            ctx.arc(n.x, n.y, n.r, 0, Math.PI * 2);
            ctx.fillStyle = `hsla(${n.hue},80%,65%,${n.alpha})`;
            ctx.fill();
        }

        requestAnimationFrame(draw);
    }
    draw();
})();

// ============================================
// TYPEWRITER
// ============================================
(function initTypewriter() {
    const el = $('hero-heading');
    if (!el) return;
    const phrases = [
        'find anything.',
        'scrape everything.',
        'know it all.',
        'dig deeper.',
    ];
    let pi = 0, ci = 0, deleting = false;
    const TYPE_SPEED = 80;
    const DELETE_SPEED = 40;
    const PAUSE = 2000;

    // Add cursor
    const cursor = document.createElement('span');
    cursor.className = 'cursor-blink';
    el.appendChild(cursor);

    function tick() {
        const phrase = phrases[pi];
        if (!deleting) {
            ci++;
            el.textContent = phrase.slice(0, ci);
            el.appendChild(cursor);
            if (ci >= phrase.length) {
                deleting = true;
                setTimeout(tick, PAUSE);
                return;
            }
            setTimeout(tick, TYPE_SPEED + Math.random() * 40);
        } else {
            ci--;
            el.textContent = phrase.slice(0, ci);
            el.appendChild(cursor);
            if (ci <= 0) {
                deleting = false;
                pi = (pi + 1) % phrases.length;
                setTimeout(tick, 300);
                return;
            }
            setTimeout(tick, DELETE_SPEED);
        }
    }
    setTimeout(tick, 600);
})();

// ============================================
// PILL TOGGLE
// ============================================
(function initToggle() {
    const pills = document.querySelectorAll('.pill');
    const slider = $('pill-slider');
    pills.forEach(p => {
        p.addEventListener('click', () => {
            const m = p.dataset.mode;
            setMode(m);
        });
    });
})();

function setMode(m) {
    mode = m;
    const slider = $('pill-slider');
    const extras = $('input-extras');
    const qInput = $('q');
    const btnText = $('go-text');

    document.querySelectorAll('.pill').forEach(p => {
        p.classList.toggle('active', p.dataset.mode === m);
    });

    if (m === 'url') {
        slider.classList.add('right');
        extras.style.display = 'flex';
        qInput.placeholder = 'paste a url — e.g. github.com';
        btnText.textContent = 'scrape';
    } else {
        slider.classList.remove('right');
        extras.style.display = 'none';
        qInput.placeholder = 'try "dolo 650" or "mechanical pencil" ...';
        btnText.textContent = 'go';
    }
}

// ============================================
// QUICK SEARCH
// ============================================
function quick(q) {
    setMode('search');
    $('q').value = q;
    fire();
}

// ============================================
// MAIN SEARCH / SCRAPE
// ============================================
async function fire() {
    const q = $('q').value.trim();
    if (!q || busy) return;

    busy = true;
    $('go').disabled = true;
    showLoader();

    try {
        let data;
        if (mode === 'search') {
            setStep(1);
            data = await fetchJSON('/api/search', { query: q, count: 10 });
            setStep(3);
        } else {
            setStep(2);
            const depth = parseInt($('depth-num').value) || 1;
            data = await fetchJSON('/api/quick-scrape', { url: q, depth });
            setStep(3);
        }
        renderResults(data.results || [], q);
    } catch (e) {
        showErr('search failed', e.message);
    } finally {
        busy = false;
        $('go').disabled = false;
        hideLoader();
    }
}

async function fetchJSON(url, body) {
    const r = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });
    if (!r.ok) {
        const err = await r.json().catch(() => ({ error: 'request failed' }));
        throw new Error(err.error || `HTTP ${r.status}`);
    }
    return r.json();
}

// ============================================
// LOADER
// ============================================
function showLoader() {
    $('loader-area').classList.add('show');
    $('results-area').classList.remove('show');
    $('err-area').classList.remove('show');
    setDot('busy', 'working');
    // reset steps
    for (let i = 1; i <= 3; i++) {
        $('ls' + i).className = 'ls';
    }
    $('ls1').classList.add('on');
}

function hideLoader() {
    $('loader-area').classList.remove('show');
}

function setStep(n) {
    for (let i = 1; i <= 3; i++) {
        const el = $('ls' + i);
        if (i < n) { el.className = 'ls done'; el.innerHTML = '&#10003; ' + el.textContent.slice(2); }
        else if (i === n) { el.className = 'ls on'; }
        else { el.className = 'ls'; }
    }
    const msgs = ['searching the web...', 'scraping pages...', 'extracting data...'];
    $('loader-msg').textContent = msgs[n - 1] || '';
}

// ============================================
// RENDER RESULTS
// ============================================
function renderResults(results, query) {
    if (!results.length) {
        showErr('nothing found', `we couldn't find results for "${query}". try different words.`);
        return;
    }

    $('results-area').classList.add('show');
    setDot('ok', results.length + ' results');

    $('rh-query').textContent = query;
    $('rh-count').textContent = results.length + ' pages scraped';

    // counters
    let tl = 0, ti = 0, tw = 0;
    results.forEach(r => {
        tl += (r.links || []).length;
        ti += (r.images || []).length;
        tw += r.word_count || 0;
    });
    animNum('c-pages', results.length);
    animNum('c-links', tl);
    animNum('c-imgs', ti);
    animNum('c-words', tw);

    // cards
    const grid = $('cards');
    grid.innerHTML = '';
    results.forEach((r, i) => {
        grid.appendChild(mkCard(r, i));
    });
}

function mkCard(r, i) {
    const card = document.createElement('div');
    card.className = 'card';
    card.style.animationDelay = i * 0.06 + 's';

    const title = r.title || 'untitled';
    const desc = r.meta_description || r.snippet || '';
    const extras = r.extras || {};
    const headings = r.headings || [];
    const sc = r.status_code || 0;

    // badge
    let badgeClass = 'badge-err', badgeText = 'err';
    if (sc >= 200 && sc < 300) { badgeClass = 'badge-ok'; badgeText = sc; }
    else if (sc >= 300 && sc < 400) { badgeClass = 'badge-warn'; badgeText = sc; }
    else if (sc >= 400) { badgeClass = 'badge-err'; badgeText = sc; }

    // extras chips
    let extrasHtml = '';
    const ek = Object.keys(extras);
    if (ek.length) {
        const chips = ek.slice(0, 6).map(k => {
            const lbl = k.replace('og:', '').replace('product:', '');
            const val = extras[k].length > 80 ? extras[k].slice(0, 80) + '…' : extras[k];
            return `<span class="ex"><span class="ex-key">${esc(lbl)}</span><span class="ex-val">${esc(val)}</span></span>`;
        }).join('');
        extrasHtml = `<div class="card-extras">${chips}</div>`;
    }

    // headings
    let hHtml = '';
    if (headings.length) {
        const items = headings.slice(0, 8).map(h => `<li>${esc(h)}</li>`).join('');
        hHtml = `<details class="card-headings"><summary>${headings.length} headings</summary><ul class="h-list">${items}</ul></details>`;
    }

    // content preview (collapsed by default)
    let contentHtml = '';
    const content = r.content || '';
    if (content.length > 0) {
        const preview = content.slice(0, 500);
        const full = content.slice(0, 3000);
        const hasMore = content.length > 500;
        const cLen = r.content_length || content.length;
        contentHtml = `
            <details class="card-content-wrap">
                <summary>page content <span class="content-len">${(cLen / 1024).toFixed(1)}KB</span></summary>
                <div class="card-content">${esc(hasMore ? preview : content)}</div>
                ${hasMore ? `<button class="show-more-btn" onclick="this.previousElementSibling.textContent=this.dataset.full;this.remove()" data-full="${esc(full).replace(/"/g, '&quot;')}">show more (${(cLen / 1024).toFixed(1)}KB total)</button>` : ''}
            </details>
        `;
    }

    card.innerHTML = `
        <div class="card-top">
            <h3 class="card-title"><a href="${esc(r.url)}" target="_blank" rel="noopener">${esc(title)}</a></h3>
            <span class="card-badge ${badgeClass}">${badgeText}</span>
        </div>
        <div class="card-url">${esc(r.url)}</div>
        ${desc ? `<p class="card-desc">${esc(desc)}</p>` : ''}
        ${extrasHtml}
        ${contentHtml}
        <div class="card-meta">
            <span class="m">${(r.links || []).length} links</span>
            <span class="m m-sep">·</span>
            <span class="m">${(r.images || []).length} imgs</span>
            <span class="m m-sep">·</span>
            <span class="m">${r.word_count || 0} words</span>
            <span class="m m-sep">·</span>
            <span class="m">${r.fetch_time_ms || 0}ms</span>
            ${r.content_length ? `<span class="m m-sep">·</span><span class="m">${(r.content_length / 1024).toFixed(1)}KB text</span>` : ''}
        </div>
        ${hHtml}
    `;

    // tilt on hover
    card.addEventListener('mousemove', e => {
        const rect = card.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        const cx = rect.width / 2;
        const cy = rect.height / 2;
        const rx = (y - cy) / cy * -3;
        const ry = (x - cx) / cx * 3;
        card.style.transform = `perspective(600px) rotateX(${rx}deg) rotateY(${ry}deg) translateY(-2px)`;
    });

    card.addEventListener('mouseleave', () => {
        card.style.transform = '';
    });

    return card;
}

// ============================================
// ERROR
// ============================================
function showErr(title, msg) {
    $('err-area').classList.add('show');
    $('results-area').classList.remove('show');
    $('err-title').textContent = title;
    $('err-msg').textContent = msg;
    setDot('err', 'error');
}

function clearResults() {
    $('results-area').classList.remove('show');
    $('err-area').classList.remove('show');
    setDot('ok', 'idle');
    $('q').focus();
}

// ============================================
// HELPERS
// ============================================
function setDot(state, label) {
    const d = $('dot');
    d.className = 'dot';
    if (state === 'busy') d.classList.add('busy');
    if (state === 'err') d.classList.add('err');
    $('dot-label').textContent = label;
}

function animNum(id, target) {
    const el = $(id);
    if (!target) { el.textContent = '0'; return; }
    let cur = 0;
    const step = Math.max(1, Math.floor(target / 18));
    const iv = setInterval(() => {
        cur += step;
        if (cur >= target) { cur = target; clearInterval(iv); }
        el.textContent = cur.toLocaleString();
    }, 35);
}

function esc(s) {
    if (!s) return '';
    const d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
}

// ============================================
// KEYBOARD
// ============================================
document.addEventListener('keydown', e => {
    if (e.key === 'Enter' && document.activeElement === $('q')) fire();
    if (e.key === 'Escape') clearResults();
    // Ctrl+K or Cmd+K to focus search
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        $('q').focus();
        $('q').select();
    }
});

// ============================================
// INIT
// ============================================
window.addEventListener('load', () => {
    $('q').focus();
});
