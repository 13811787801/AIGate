(() => {
  const state = {
    token: localStorage.getItem("aigate_token") || "",
    user: null,
    page: "overview",
  };

  const $ = (id) => document.getElementById(id);
  const viewLogin = $("view-login");
  const viewShell = $("view-shell");
  const pageEl = $("page");
  const pageTitle = $("page-title");
  const titles = {
    overview: "总览",
    chat: "对话调试",
    keys: "令牌管理",
    logs: "使用日志",
    wallet: "钱包",
    channels: "渠道配置",
    models: "模型配置",
  };

  async function api(path, opts = {}) {
    const headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
    if (state.token) headers.Authorization = "Bearer " + state.token;
    const res = await fetch(path, { ...opts, headers });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      const msg = data.error || data.message || res.statusText;
      throw new Error(typeof msg === "string" ? msg : JSON.stringify(msg));
    }
    return data;
  }

  function showLogin() {
    viewLogin.classList.remove("hidden");
    viewShell.classList.add("hidden");
  }

  function showShell() {
    viewLogin.classList.add("hidden");
    viewShell.classList.remove("hidden");
    $("side-user").textContent = state.user
      ? `${state.user.username}${state.user.is_admin ? " · 管理员" : ""}`
      : "—";
    document.querySelectorAll(".admin-only").forEach((el) => {
      el.style.display = state.user?.is_admin ? "" : "none";
    });
    refreshBalance();
    render();
  }

  async function refreshBalance() {
    try {
      const w = await api("/api/wallet");
      $("balance-pill").textContent = `余额 ¥${Number(w.balance).toFixed(4)}`;
      if (state.user) {
        state.user.balance = w.balance;
        state.user.frozen = w.frozen;
        state.user.total_consumed = w.total_consumed;
      }
    } catch (_) {}
  }

  async function boot() {
    if (!state.token) {
      showLogin();
      return;
    }
    try {
      state.user = await api("/api/me");
      showShell();
    } catch (_) {
      state.token = "";
      localStorage.removeItem("aigate_token");
      showLogin();
    }
  }

  $("btn-login").onclick = async () => {
    $("login-error").textContent = "";
    try {
      const data = await api("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({
          username: $("login-user").value.trim(),
          password: $("login-pass").value,
        }),
      });
      state.token = data.token;
      state.user = data.user;
      localStorage.setItem("aigate_token", state.token);
      showShell();
    } catch (e) {
      $("login-error").textContent = e.message;
    }
  };

  $("btn-logout").onclick = () => {
    state.token = "";
    state.user = null;
    localStorage.removeItem("aigate_token");
    showLogin();
  };

  document.querySelectorAll("#nav button[data-page]").forEach((btn) => {
    btn.onclick = () => {
      state.page = btn.dataset.page;
      render();
    };
  });

  function setActiveNav() {
    document.querySelectorAll("#nav button[data-page]").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.page === state.page);
    });
    pageTitle.textContent = titles[state.page] || state.page;
  }

  async function render() {
    setActiveNav();
    pageEl.innerHTML = `<div class="panel"><p class="hint">加载中…</p></div>`;
    try {
      if (state.page === "overview") await renderOverview();
      else if (state.page === "chat") await renderChat();
      else if (state.page === "keys") await renderKeys();
      else if (state.page === "logs") await renderLogs();
      else if (state.page === "wallet") await renderWallet();
      else if (state.page === "channels") await renderChannels();
      else if (state.page === "models") await renderModels();
    } catch (e) {
      pageEl.innerHTML = `<div class="panel"><p class="error">${esc(e.message)}</p></div>`;
    }
  }

  async function renderOverview() {
    await refreshBalance();
    let adminCards = "";
    if (state.user?.is_admin) {
      const s = await api("/api/admin/overview");
      adminCards = `
        <div class="card"><div class="label">用户数</div><div class="value">${s.users}</div></div>
        <div class="card"><div class="label">令牌数</div><div class="value">${s.api_keys}</div></div>
        <div class="card"><div class="label">渠道数</div><div class="value">${s.channels}</div></div>
        <div class="card"><div class="label">模型数</div><div class="value">${s.models}</div></div>
        <div class="card"><div class="label">请求日志</div><div class="value">${s.request_logs}</div></div>
        <div class="card"><div class="label">累计消费</div><div class="value">${Number(s.total_consumed).toFixed(4)}</div></div>`;
    }
    pageEl.innerHTML = `
      <div class="cards">
        <div class="card"><div class="label">当前余额</div><div class="value">${Number(state.user.balance).toFixed(4)}</div></div>
        <div class="card"><div class="label">冻结中</div><div class="value">${Number(state.user.frozen || 0).toFixed(4)}</div></div>
        <div class="card"><div class="label">累计消费</div><div class="value">${Number(state.user.total_consumed || 0).toFixed(4)}</div></div>
        ${adminCards}
      </div>
      <div class="panel">
        <h3>接入说明</h3>
        <p class="hint">OpenAI 兼容 Base URL：</p>
        <p class="mono">http://&lt;你的服务器&gt;:3000/v1</p>
        <p class="hint" style="margin-top:10px">在「令牌管理」创建 Key 后，即可用官方 SDK / Cursor 调用 ChatGPT、DeepSeek 等（先在管理端配置对应渠道与模型）。</p>
        <p class="hint">内置 <code>mock-chat</code> 模型可离线验证全链路。</p>
      </div>`;
  }

  async function renderChat() {
    const models = await api("/api/models");
    const opts = (models.items || [])
      .map((m) => `<option value="${esc(m.ModelName || m.model_name)}">${esc(m.DisplayName || m.display_name || m.ModelName || m.model_name)}</option>`)
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <div class="row">
          <div>
            <label>模型</label>
            <select id="chat-model">${opts || '<option value="mock-chat">mock-chat</option>'}</select>
          </div>
        </div>
        <label>消息</label>
        <textarea id="chat-input" placeholder="输入内容，例如：你好">你好，请做个简短自我介绍</textarea>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary" id="chat-send" style="width:auto">发送</button>
        </div>
        <label style="margin-top:16px">回复</label>
        <div class="msg" id="chat-out">—</div>
      </div>`;
    $("chat-send").onclick = async () => {
      const out = $("chat-out");
      out.textContent = "请求中…";
      try {
        const data = await api("/api/chat", {
          method: "POST",
          body: JSON.stringify({
            model: $("chat-model").value,
            messages: [{ role: "user", content: $("chat-input").value }],
            stream: false,
          }),
        });
        const content = data.choices?.[0]?.message?.content || JSON.stringify(data);
        out.textContent = content;
        refreshBalance();
      } catch (e) {
        out.innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
  }

  async function renderKeys() {
    const data = await api("/api/keys");
    const rows = (data.items || [])
      .map(
        (k) => `<tr>
        <td>${k.id}</td>
        <td>${esc(k.name)}</td>
        <td class="mono">${esc(k.key_prefix)}</td>
        <td>${k.status === 1 ? '<span class="ok">启用</span>' : "停用"}</td>
        <td>${k.rpm_limit} / ${k.tpm_limit}</td>
        <td>${k.quota_used}</td>
        <td><button class="btn danger" data-del="${k.id}">删除</button></td>
      </tr>`
      )
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>创建令牌</h3>
        <div class="row">
          <div><label>名称</label><input id="key-name" placeholder="my-app" /></div>
          <div><label>RPM</label><input id="key-rpm" type="number" value="60" /></div>
          <div><label>TPM</label><input id="key-tpm" type="number" value="100000" /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary" id="key-create" style="width:auto">创建</button>
        </div>
        <p class="hint" id="key-created"></p>
      </div>
      <div class="panel">
        <h3>我的令牌</h3>
        <table class="table">
          <thead><tr><th>ID</th><th>名称</th><th>前缀</th><th>状态</th><th>RPM/TPM</th><th>已用Token</th><th></th></tr></thead>
          <tbody>${rows || '<tr><td colspan="7">暂无</td></tr>'}</tbody>
        </table>
      </div>`;
    $("key-create").onclick = async () => {
      try {
        const res = await api("/api/keys", {
          method: "POST",
          body: JSON.stringify({
            name: $("key-name").value || "default",
            rpm_limit: Number($("key-rpm").value || 60),
            tpm_limit: Number($("key-tpm").value || 100000),
          }),
        });
        $("key-created").innerHTML = `已创建，请立即复制完整密钥：<br/><code class="mono">${esc(res.api_key)}</code>`;
        renderKeys();
      } catch (e) {
        $("key-created").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
    pageEl.querySelectorAll("[data-del]").forEach((btn) => {
      btn.onclick = async () => {
        if (!confirm("确认删除该令牌？")) return;
        await api("/api/keys?id=" + btn.dataset.del, { method: "DELETE" });
        renderKeys();
      };
    });
  }

  async function renderLogs() {
    const data = await api("/api/logs");
    const rows = (data.items || [])
      .map(
        (l) => `<tr>
        <td class="mono">${esc(l.CreatedAt || l.created_at || "")}</td>
        <td>${esc(l.ModelName || l.model_name || "")}</td>
        <td>${l.InputTokens ?? l.input_tokens ?? 0} / ${l.OutputTokens ?? l.output_tokens ?? 0}</td>
        <td>${Number(l.Cost ?? l.cost ?? 0).toFixed(6)}</td>
        <td>${(l.Success ?? l.success) ? '<span class="ok">成功</span>' : '<span class="bad">失败</span>'}</td>
        <td>${l.LatencyMs ?? l.latency_ms ?? 0}ms</td>
        <td class="mono">${esc(l.TraceID || l.trace_id || "")}</td>
      </tr>`
      )
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <table class="table">
          <thead><tr><th>时间</th><th>模型</th><th>In/Out</th><th>费用</th><th>结果</th><th>耗时</th><th>Trace</th></tr></thead>
          <tbody>${rows || '<tr><td colspan="7">暂无日志</td></tr>'}</tbody>
        </table>
      </div>`;
  }

  async function renderWallet() {
    await refreshBalance();
    pageEl.innerHTML = `
      <div class="cards">
        <div class="card"><div class="label">可用余额</div><div class="value">${Number(state.user.balance).toFixed(6)}</div></div>
        <div class="card"><div class="label">冻结</div><div class="value">${Number(state.user.frozen || 0).toFixed(6)}</div></div>
        <div class="card"><div class="label">累计消费</div><div class="value">${Number(state.user.total_consumed || 0).toFixed(6)}</div></div>
      </div>
      <div class="panel"><p class="hint">Phase 2 暂用预置余额。在线充值 / 兑换码将在后续阶段加入。</p></div>`;
  }

  async function renderChannels() {
    const data = await api("/api/admin/console/channels");
    const rows = (data.items || [])
      .map(
        (c) => `<tr>
        <td>${c.id}</td><td>${esc(c.name)}</td><td>${esc(c.provider)}</td>
        <td class="mono">${esc(c.base_url)}</td><td class="mono">${esc(c.api_key_masked)}</td>
        <td>${c.weight}/${c.priority}</td><td>${c.status === 1 ? "启用" : "停用"}</td>
      </tr>`
      )
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>新增渠道（OpenAI 兼容：ChatGPT / DeepSeek 等）</h3>
        <div class="row">
          <div><label>名称</label><input id="ch-name" placeholder="deepseek-main" /></div>
          <div><label>Provider</label>
            <select id="ch-provider">
              <option value="openai_compatible">openai_compatible</option>
              <option value="mock">mock</option>
            </select>
          </div>
        </div>
        <div class="row">
          <div><label>Base URL</label><input id="ch-url" placeholder="https://api.deepseek.com/v1" /></div>
          <div><label>上游 API Key</label><input id="ch-key" placeholder="sk-..." /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary" id="ch-save" style="width:auto">保存渠道</button>
        </div>
        <p class="hint" id="ch-msg"></p>
      </div>
      <div class="panel">
        <h3>渠道列表</h3>
        <table class="table">
          <thead><tr><th>ID</th><th>名称</th><th>类型</th><th>Base URL</th><th>Key</th><th>权重/优先级</th><th>状态</th></tr></thead>
          <tbody>${rows || '<tr><td colspan="7">暂无</td></tr>'}</tbody>
        </table>
      </div>`;
    $("ch-save").onclick = async () => {
      try {
        const res = await api("/api/admin/console/channels", {
          method: "POST",
          body: JSON.stringify({
            name: $("ch-name").value,
            provider: $("ch-provider").value,
            base_url: $("ch-url").value,
            api_key: $("ch-key").value,
            weight: 100,
            priority: 10,
          }),
        });
        $("ch-msg").innerHTML = `<span class="ok">已创建渠道 ID=${res.id}</span>`;
        renderChannels();
      } catch (e) {
        $("ch-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
  }

  async function renderModels() {
    const [models, channels] = await Promise.all([
      api("/api/admin/console/models"),
      api("/api/admin/console/channels"),
    ]);
    const chOpts = (channels.items || [])
      .map((c) => `<option value="${c.id}">#${c.id} ${esc(c.name)} (${esc(c.provider)})</option>`)
      .join("");
    const rows = (models.items || [])
      .map((m) => {
        const name = m.ModelName || m.model_name;
        const disp = m.DisplayName || m.display_name || name;
        const inP = m.InputPricePer1K ?? m.input_price_per_1k ?? 0;
        const outP = m.OutputPricePer1K ?? m.output_price_per_1k ?? 0;
        const en = m.Enabled ?? m.enabled;
        return `<tr><td>${m.ID || m.id}</td><td>${esc(name)}</td><td>${esc(disp)}</td>
          <td>${inP} / ${outP}</td><td>${en ? "启用" : "停用"}</td></tr>`;
      })
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>新增对外模型并绑定渠道</h3>
        <div class="row">
          <div><label>对外模型名</label><input id="m-name" placeholder="deepseek-chat" /></div>
          <div><label>显示名</label><input id="m-disp" placeholder="DeepSeek Chat" /></div>
        </div>
        <div class="row">
          <div><label>上游模型名</label><input id="m-up" placeholder="deepseek-chat" /></div>
          <div><label>绑定渠道</label><select id="m-ch">${chOpts || "<option value=''>请先创建渠道</option>"}</select></div>
        </div>
        <div class="row">
          <div><label>输入价 / 1K</label><input id="m-in" type="number" step="0.0001" value="0.001" /></div>
          <div><label>输出价 / 1K</label><input id="m-out" type="number" step="0.0001" value="0.002" /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary" id="m-save" style="width:auto">保存模型</button>
        </div>
        <p class="hint" id="m-msg"></p>
      </div>
      <div class="panel">
        <h3>模型列表</h3>
        <table class="table">
          <thead><tr><th>ID</th><th>模型名</th><th>显示名</th><th>价格 In/Out</th><th>状态</th></tr></thead>
          <tbody>${rows || '<tr><td colspan="5">暂无</td></tr>'}</tbody>
        </table>
      </div>`;
    $("m-save").onclick = async () => {
      try {
        const res = await api("/api/admin/console/models", {
          method: "POST",
          body: JSON.stringify({
            model_name: $("m-name").value,
            display_name: $("m-disp").value || $("m-name").value,
            upstream_model: $("m-up").value || $("m-name").value,
            channel_id: Number($("m-ch").value),
            input_price_per_1k: Number($("m-in").value),
            output_price_per_1k: Number($("m-out").value),
            group_id: 1,
          }),
        });
        $("m-msg").innerHTML = `<span class="ok">已创建模型 ID=${res.id}</span>`;
        renderModels();
      } catch (e) {
        $("m-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
  }

  function esc(s) {
    return String(s ?? "")
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
  }

  // Go json 默认导出字段名可能是大写，做兼容已在上面处理
  boot();
})();
