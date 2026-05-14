const qualityMap = {
  jpg: { '4': '4 (High)', '8': '8 (Mid)', '16': '16 (Low)' },
  avif: { '20': '20 (High)', '30': '30 (Mid)', '40': '40 (Low)' },
  libsvtav1: { '4': '4 (Slow)', '7': '7 (Mid)', '10': '10 (Fast)' },
  libx265: { '14': '14 (High)', '28': '28 (Mid)', '42': '42 (Low)' },
};

const defaultBitrates = {
  libsvtav1: '2500k',
  libx265: '3500k',
  hevc_mediacodec: '3500k',
};

let config = {};
let eventSource = null;

async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch(path, opts);
  return res.json();
}

function updateQualityDropdown(target, codec) {
  const map = qualityMap[codec];
  target.innerHTML = '';
  if (!map) { target.parentElement.style.display = 'none'; return; }
  target.parentElement.style.display = '';
  for (const [val, label] of Object.entries(map)) {
    const opt = document.createElement('option');
    opt.value = val; opt.textContent = label;
    target.appendChild(opt);
  }
}

function collectExtTags(containerId) {
  const tags = [];
  document.querySelectorAll(`#${containerId} .tag-remove`).forEach(el => {
    tags.push(el.dataset.ext);
  });
  return tags;
}

function collectConfig() {
  return {
    input_path: document.getElementById('input-path').value,
    output_path: document.getElementById('output-path').value,
    bind_address: document.getElementById('bind-address').value,
    port: document.getElementById('bind-port').value,
    image_codec: document.getElementById('image-codec').value,
    image_quality: parseInt(document.getElementById('image-quality').value),
    avif_cpu: parseInt(document.getElementById('avif-cpu').value),
    video_codec: document.getElementById('video-codec').value,
    video_bitrate: document.getElementById('video-bitrate').value,
    audio_codec: 'libopus',
    audio_bitrate: document.getElementById('audio-bitrate').value,
    video_quality: document.getElementById('video-quality').value,
    delete_original: document.getElementById('delete-original').checked,
    window_start: document.getElementById('window-start').value,
    window_end: document.getElementById('window-end').value,
    image_extensions: collectExtTags('image-ext-tags'),
    video_extensions: collectExtTags('video-ext-tags'),
  };
}

function applyConfig(cfg) {
  config = cfg;
  document.getElementById('input-path').value = cfg.input_path || '~/storage/dcim/Camera';
  document.getElementById('output-path').value = cfg.output_path || '~/storage/dcim/slimr';
  document.getElementById('bind-address').value = cfg.bind_address || '127.0.0.1';
  document.getElementById('bind-port').value = cfg.port || '8880';
  document.getElementById('image-codec').value = cfg.image_codec || 'avif';
  document.getElementById('avif-cpu').value = cfg.avif_cpu || 4;
  document.getElementById('video-codec').value = cfg.video_codec || 'hevc_mediacodec';
  document.getElementById('video-bitrate').value = cfg.video_bitrate || '3500k';
  document.getElementById('audio-bitrate').value = cfg.audio_bitrate || '96k';
  document.getElementById('delete-original').checked = cfg.delete_original || false;
  document.getElementById('window-start').value = cfg.window_start || '23:00';
  document.getElementById('window-end').value = cfg.window_end || '07:00';

  updateQualityDropdown(document.getElementById('image-quality'), cfg.image_codec || 'avif');
  document.getElementById('image-quality').value = cfg.image_quality || '30';

  updateQualityDropdown(document.getElementById('video-quality'), cfg.video_codec || 'hevc_mediacodec');
  if (cfg.video_quality) document.getElementById('video-quality').value = cfg.video_quality;

  document.getElementById('avif-cpu-row').style.display = (cfg.image_codec === 'avif') ? '' : 'none';

  if (cfg.image_extensions) renderExtTags('image-ext-tags', cfg.image_extensions);
  if (cfg.video_extensions) renderExtTags('video-ext-tags', cfg.video_extensions);
}

function renderExtTags(containerId, exts) {
  const container = document.getElementById(containerId);
  container.innerHTML = '';
  for (const ext of exts) {
    const span = document.createElement('span');
    span.className = 'tag';
    span.innerHTML = `${ext}<span class="tag-remove" data-ext="${ext}">×</span>`;
    container.appendChild(span);
  }
  attachTagRemoveListeners(containerId);
}

function attachTagRemoveListeners(containerId) {
  document.querySelectorAll(`#${containerId} .tag-remove`).forEach(el => {
    el.onclick = () => { el.parentElement.remove(); };
  });
}

function addExtTag(containerId, inputId) {
  const input = document.getElementById(inputId);
  const ext = input.value.trim().toLowerCase();
  if (!ext.startsWith('.')) { input.value = ''; return; }
  const existing = collectExtTags(containerId);
  if (existing.includes(ext)) { input.value = ''; return; }
  const container = document.getElementById(containerId);
  const span = document.createElement('span');
  span.className = 'tag';
  span.innerHTML = `${ext}<span class="tag-remove" data-ext="${ext}">×</span>`;
  container.appendChild(span);
  attachTagRemoveListeners(containerId);
  input.value = '';
}

let currentRunning = false;

function setToggleButton(running) {
  const btn = document.getElementById('btn-toggle');
  currentRunning = running;
  if (running) {
    btn.textContent = '⏹ Stop';
    btn.className = 'btn btn-stop';
  } else {
    btn.textContent = '▶ Start';
    btn.className = 'btn btn-start';
  }
}

function setStatus(state, filesDone, filesTotal) {
  const dot = document.getElementById('status-dot');
  dot.className = `dot ${state}`;
  document.getElementById('status-text').textContent = state;
  document.getElementById('status-files').textContent = `${filesDone || 0}/${filesTotal || 0}`;
}

async function loadConfig() {
  try {
    const cfg = await api('GET', '/api/config');
    applyConfig(cfg);
  } catch (e) { console.error('load config:', e); }
}

async function pollStatus() {
  try {
    const st = await api('GET', '/api/status');
    setStatus(st.state, st.files_done, st.files_total);
    document.getElementById('status-current').textContent = st.current_file ? ` | ${st.current_file}` : '';
    document.getElementById('status-last-run').textContent = st.last_run || '—';
    document.getElementById('status-version').textContent = st.version || '—';
    if (currentRunning !== st.running) {
      setToggleButton(st.running);
    }
  } catch (e) {}
}

function connectSSE() {
  if (eventSource) eventSource.close();
  eventSource = new EventSource('/api/logs/stream');
  eventSource.onmessage = (e) => {
    try {
      const data = JSON.parse(e.data);
      if (data.type === 'log') {
        appendLog(data.line);
      } else if (data.type === 'progress') {
        updateProgress(data);
      } else if (data.type === 'status') {
        setStatus(data.state, data.files_done, data.files_total);
      }
    } catch (_) {}
  };
  eventSource.onerror = () => {
    setTimeout(connectSSE, 3000);
  };
}

function appendLog(line) {
  const el = document.getElementById('log-content');
  if (el) {
    el.textContent += line + '\n';
    el.scrollTop = el.scrollHeight;
  }
}

function updateProgress(data) {
  document.getElementById('status-current').textContent =
    data.file ? ` | ${data.file} (${data.elapsed.toFixed(1)}s)` : '';
}

async function saveConfig() {
  const cfg = collectConfig();
  await api('PUT', '/api/config', cfg);
  applyConfig(cfg);
}

async function toggleWorker() {
  const btn = document.getElementById('btn-toggle');
  btn.disabled = true;
  if (currentRunning) {
    await api('POST', '/api/stop');
  } else {
    await api('POST', '/api/start');
  }
  btn.disabled = false;
  pollStatus();
}

function openLogs() {
  const panel = document.getElementById('log-panel');
  panel.classList.remove('hidden');
  fetch('/api/logs?tail=200')
    .then(r => r.text())
    .then(text => {
      document.getElementById('log-content').textContent = text || '';
      document.getElementById('log-content').scrollTop = document.getElementById('log-content').scrollHeight;
    })
    .catch(() => {});
}

function closeLogs() {
  document.getElementById('log-panel').classList.add('hidden');
}

function clearLogs() {
  document.getElementById('log-content').textContent = '';
}

function init() {
  loadConfig();
  connectSSE();
  setInterval(pollStatus, 2000);

  document.getElementById('btn-save').onclick = saveConfig;
  document.getElementById('btn-toggle').onclick = toggleWorker;
  document.getElementById('btn-logs').onclick = openLogs;
  document.getElementById('btn-close-logs').onclick = closeLogs;
  document.getElementById('btn-clear-logs').onclick = clearLogs;

  document.getElementById('image-codec').onchange = function() {
    updateQualityDropdown(document.getElementById('image-quality'), this.value);
    document.getElementById('avif-cpu-row').style.display = (this.value === 'avif') ? '' : 'none';
  };

  document.getElementById('video-codec').onchange = function() {
    updateQualityDropdown(document.getElementById('video-quality'), this.value);
    document.getElementById('video-bitrate').value = defaultBitrates[this.value] || '3500k';
  };

  document.getElementById('image-ext-add').onkeydown = function(e) {
    if (e.key === 'Enter') { addExtTag('image-ext-tags', 'image-ext-add'); }
  };
  document.getElementById('video-ext-add').onkeydown = function(e) {
    if (e.key === 'Enter') { addExtTag('video-ext-tags', 'video-ext-add'); }
  };

  attachTagRemoveListeners('image-ext-tags');
  attachTagRemoveListeners('video-ext-tags');

  document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') closeLogs();
  });
}

document.addEventListener('DOMContentLoaded', init);
