const http = require('http');
const { execSync } = require('child_process');
const PORT = 3000;
const DB_IP = '172.22.0.10';

function queryAmericans() {
  const raw = execSync(
    'mysql -h ' + DB_IP + ' -u app -papppass -D app --skip-ssl --default-character-set=utf8mb4 ' +
    '-e "SELECT FirstName, LastName, StateEmoji, StateOfOrigin FROM famous_americans ORDER BY RAND() LIMIT 3" ' +
    '--batch --raw 2>/dev/null',
    { timeout: 10000, encoding: 'utf8' }
  );
  const lines = raw.trim().split('\n');
  const headers = lines[0].split('\t');
  return lines.slice(1).map(l => {
    const v = l.split('\t');
    return Object.fromEntries(headers.map((h, i) => [h, v[i]]));
  });
}

const server = http.createServer((req, res) => {
  try {
    const rows = queryAmericans();
    const tableRows = rows.map(r =>
      '<tr><td>' + r.FirstName + '</td><td>' + r.LastName + '</td><td>' + r.StateEmoji + ' ' + r.StateOfOrigin + '</td></tr>'
    ).join('\n');
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end('<!DOCTYPE html>\n' +
'<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">\n' +
'<title>Insecure Demo \u2014 US Viewer</title>\n' +
'<style>\n' +
'  * { margin: 0; padding: 0; box-sizing: border-box; }\n' +
'  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;\n' +
'         background: #fff5f5; display: flex; justify-content: center; align-items: center;\n' +
'         min-height: 100vh; color: #1a202c; }\n' +
'  .card { background: #fff; border-radius: 12px; box-shadow: 0 4px 24px rgba(0,0,0,.08);\n' +
'          padding: 2rem 2.5rem; max-width: 600px; width: 100%; }\n' +
'  h1 { font-size: 1.5rem; margin-bottom: .25rem; }\n' +
'  .badge { display: inline-block; background: #fed7d7; color: #9b2c2c; font-size: .7rem;\n' +
'           padding: .15rem .5rem; border-radius: 4px; margin-left: .5rem; vertical-align: middle;\n' +
'           font-weight: 600; letter-spacing: .03em; }\n' +
'  p.sub { color: #718096; margin-bottom: 1.5rem; font-size: .9rem; }\n' +
'  table { width: 100%; border-collapse: collapse; }\n' +
'  th { text-align: left; padding: .75rem 1rem; background: #fff5f5; color: #c53030;\n' +
'       font-size: .8rem; text-transform: uppercase; letter-spacing: .05em; }\n' +
'  td { padding: .75rem 1rem; border-bottom: 1px solid #fed7d7; }\n' +
'  tr:last-child td { border-bottom: none; }\n' +
'  .timer { margin-top: 1.5rem; text-align: center; }\n' +
'  .timer-label { font-size: .8rem; color: #718096; margin-bottom: .5rem; }\n' +
'  .timer-bar { height: 4px; background: #fed7d7; border-radius: 2px; overflow: hidden; }\n' +
'  .timer-fill { height: 100%; background: #e53e3e; border-radius: 2px;\n' +
'                width: 100%; animation: countdown 5s linear forwards; }\n' +
'  .timer-fill.paused { animation-play-state: paused; }\n' +
'  .pause-btn { margin-top: .75rem; padding: .35rem 1rem; background: #fed7d7; color: #9b2c2c;\n' +
'              border: none; border-radius: 6px; font-size: .8rem; cursor: pointer; }\n' +
'  .pause-btn:hover { background: #feb2b2; }\n' +
'  @keyframes countdown { from { width: 100%; } to { width: 0%; } }\n' +
'</style></head><body>\n' +
'<div class="card">\n' +
'  <h1>Famous Americans <span class="badge">\u26a0 INSECURE \u2014 HTTP</span></h1>\n' +
'  <p class="sub">3 random selections from the local US database (no Tailscale, no encryption)</p>\n' +
'  <table>\n' +
'    <thead><tr><th>First Name</th><th>Last Name</th><th>State</th></tr></thead>\n' +
'    <tbody>' + tableRows + '</tbody>\n' +
'  </table>\n' +
'  <div class="timer">\n' +
'    <div class="timer-label">Refreshing in <span id="sec">5</span>s</div>\n' +
'    <div class="timer-bar"><div class="timer-fill" id="bar"></div></div>\n' +
'    <button class="pause-btn" id="pauseBtn">Pause</button>\n' +
'  </div>\n' +
'</div>\n' +
'<script>\n' +
'  let t=5, paused=false;\n' +
'  const el=document.getElementById("sec"), bar=document.getElementById("bar"), btn=document.getElementById("pauseBtn");\n' +
'  btn.onclick=()=>{ paused=!paused; btn.textContent=paused?"Resume":"Pause";\n' +
'    bar.classList.toggle("paused",paused); };\n' +
'  setInterval(()=>{ if(paused) return; t--; el.textContent=t; if(t<=0) location.reload(); },1000);\n' +
'</script>\n' +
'</body></html>');
  } catch (err) {
    res.writeHead(500, { 'Content-Type': 'text/html' });
    res.end('<h1>Error</h1><pre>' + err.message + '</pre>');
  }
});
server.listen(PORT, () => console.log('Insecure Demo US Viewer listening on ' + PORT));
