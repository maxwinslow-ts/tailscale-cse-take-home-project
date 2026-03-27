const http = require('http');
const { execSync } = require('child_process');
const PORT = 8080;
const DB_IP = process.env.EU_DB_BRIDGE_IP;
const CHECK_INTERVAL = 5000;

let lastResults = { ts: null, checks: [] };

function run(cmd, timeout) {
  try {
    return { ok: true, out: execSync(cmd, { timeout: timeout || 8000, encoding: 'utf8' }).trim() };
  } catch (e) {
    return { ok: false, out: (e.stderr || e.message || '').toString().trim() };
  }
}

function ping(ip) {
  return run('ping -c 1 -W 3 ' + ip + ' 2>/dev/null', 5000);
}

function latency(r) {
  if (!r.ok) return null;
  const m = r.out.match(/time[=<](\d+\.?\d*)/); return m ? m[1] + ' ms' : null;
}

function runChecks() {
  const checks = [];

  // 1. Ping euroviewer app container (bridge)
  const p1 = ping('172.21.0.10');
  checks.push({ name: 'Euroviewer App', desc: 'Ping 172.21.0.10 (Docker bridge)',
    pass: p1.ok, detail: p1.ok ? latency(p1) || 'reachable' : 'unreachable' });

  // 2. Ping euroviewer-ts Tailscale sidecar (bridge)
  const p2 = ping('172.21.0.2');
  checks.push({ name: 'Tailscale Sidecar', desc: 'Ping 172.21.0.2 (Docker bridge)',
    pass: p2.ok, detail: p2.ok ? latency(p2) || 'reachable' : 'unreachable' });

  // 3. Ping eu-db bridge IP (through subnet route)
  const p4 = ping(DB_IP);
  checks.push({ name: 'EU Database', desc: 'Ping ' + DB_IP + ' via subnet route',
    pass: p4.ok, detail: p4.ok ? latency(p4) || 'reachable' : 'unreachable' });

  // 4. MySQL query (full data path)
  const mysql = run(
    'mysql -h ' + DB_IP + ' -u app -papppass -D app --skip-ssl --default-character-set=utf8mb4 ' +
    '-e "SELECT COUNT(*) AS cnt FROM famous_europeans" --batch --raw 2>/dev/null'
  );
  const rowCount = mysql.ok ? (mysql.out.split('\n')[1] || '0') : '0';
  checks.push({ name: 'MySQL Query', desc: 'SELECT from eu-db via subnet route',
    pass: mysql.ok && parseInt(rowCount) > 0,
    detail: mysql.ok ? rowCount + ' rows' : 'connection failed' });

  // 5. HTTP /json on euroviewer
  const appJson = run('curl -sf --max-time 5 http://172.21.0.10:3000/json 2>/dev/null');
  let jsonOk = false;
  if (appJson.ok) { try { jsonOk = JSON.parse(appJson.out).source === 'eu-db'; } catch(e) {} }
  checks.push({ name: 'Euroviewer /json', desc: 'HTTP JSON endpoint returns eu-db data',
    pass: jsonOk, detail: jsonOk ? 'OK' : 'no response' });

  // 6. HTTP / on euroviewer
  const appHtml = run('curl -sf --max-time 5 http://172.21.0.10:3000/ 2>/dev/null');
  const htmlOk = appHtml.ok && /Famous Europeans/.test(appHtml.out);
  checks.push({ name: 'Euroviewer HTML', desc: 'Root endpoint renders table',
    pass: htmlOk, detail: htmlOk ? 'OK' : 'no response' });

  const passed = checks.filter(c => c.pass).length;
  lastResults = { ts: new Date().toISOString(), checks, passed, total: checks.length };
}

runChecks();
setInterval(runChecks, CHECK_INTERVAL);

const server = http.createServer((req, res) => {
  if (req.url === '/api') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(lastResults));
    return;
  }

  const r = lastResults;
  const allGreen = r.passed === r.total;
  const checksHtml = (r.checks || []).map(c =>
    '<tr class="' + (c.pass ? 'pass' : 'fail') + '">' +
    '<td class="status">' + (c.pass ? '\u2705' : '\u274C') + '</td>' +
    '<td><strong>' + c.name + '</strong><br><span class="desc">' + c.desc + '</span></td>' +
    '<td class="detail">' + c.detail + '</td></tr>'
  ).join('\n');

  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
  res.end('<!DOCTYPE html>\n' +
    '<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">\n' +
    '<title>Health Check</title>\n' +
    '<meta http-equiv="refresh" content="5">\n' +
    '<style>\n' +
    '  * { margin:0; padding:0; box-sizing:border-box; }\n' +
    '  body { font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;\n' +
    '         background:#0f172a; color:#e2e8f0; min-height:100vh; padding:2rem; }\n' +
    '  .wrap { max-width:700px; margin:0 auto; }\n' +
    '  .header { display:flex; align-items:center; gap:1rem; margin-bottom:1.5rem; }\n' +
    '  .header .dot { width:16px; height:16px; border-radius:50%;\n' +
    '                 background:' + (allGreen ? '#22c55e' : '#ef4444') + ';\n' +
    '                 box-shadow:0 0 8px ' + (allGreen ? '#22c55e88' : '#ef444488') + '; }\n' +
    '  h1 { font-size:1.25rem; font-weight:600; }\n' +
    '  .meta { color:#64748b; font-size:.8rem; margin-bottom:1.5rem; display:flex; align-items:center; gap:.75rem; }\n' +
    '  .timer-bar { flex:1; max-width:120px; height:4px; background:#1e293b; border-radius:2px; overflow:hidden; }\n' +
    '  .timer-fill { height:100%; background:' + (allGreen ? '#22c55e' : '#f59e0b') + ';\n' +
    '                border-radius:2px; animation:countdown 5s linear forwards; }\n' +
    '  @keyframes countdown { from { width:100%; } to { width:0%; } }\n' +
    '  .score { font-size:.9rem; margin-bottom:1rem;\n' +
    '           color:' + (allGreen ? '#22c55e' : '#f59e0b') + '; font-weight:600; }\n' +
    '  table { width:100%; border-collapse:collapse; }\n' +
    '  tr { border-bottom:1px solid #1e293b; }\n' +
    '  td { padding:.75rem .5rem; vertical-align:top; }\n' +
    '  .status { width:2rem; text-align:center; font-size:1.1rem; }\n' +
    '  .desc { color:#64748b; font-size:.8rem; }\n' +
    '  .detail { color:#94a3b8; font-size:.85rem; text-align:right; }\n' +
    '  tr.fail { background:#1c1117; }\n' +
    '  tr.pass { background:#0f1a12; }\n' +
    '  .footer { margin-top:1.5rem; text-align:center; font-size:.75rem; color:#475569; }\n' +
    '  .footer a { color:#64748b; }\n' +
    '</style></head><body>\n' +
    '<div class="wrap">\n' +
    '  <div class="header"><div class="dot"></div><h1>Sovereign Mesh \u2014 Health Check</h1></div>\n' +
    '  <div class="meta">Last check: ' + (r.ts || 'pending') + ' <div class="timer-bar"><div class="timer-fill"></div></div> <span id="cd">5</span>s</div>\n' +
    '  <div class="score">' + (r.passed || 0) + ' / ' + (r.total || 0) + ' checks passing</div>\n' +
    '  <table>' + checksHtml + '</table>\n' +
    '  <div class="footer"><a href="/api">JSON API</a> \u00B7 <a href="http://' + req.headers.host.replace(':8080', '') + '/">Euroviewer</a></div>\n' +
    '</div>\n' +
    '<script>let s=5;const e=document.getElementById("cd");const t=setInterval(()=>{s--;if(s<0)s=0;e.textContent=s;},1000);</script>\n' +
    '</body></html>');
});
server.listen(PORT, () => console.log('Health check dashboard on ' + PORT));
