(() => {
  const state = {
    token: localStorage.getItem("aigate_token") || "",
    user: null,
    page: "overview",
    modelsCache: [],
  };

  const $ = (id) => document.getElementById(id);
  const viewLogin = $("view-login");
  const viewShell = $("view-shell");
  const pageEl = $("page");
  const pageTitle = $("page-title");
  const pageSub = $("page-sub");

  const meta = {
    overview: ["数据看板", "账户用量与平台概览"],
    marketplace: ["模型广场", "浏览可用模型、计费方式与单价（对标商业货架）"],
    chat: ["操练场", "在线调试模型调用（流式前先用非流式验证）"],
    keys: ["令牌管理", "创建业务系统使用的 API Key"],
    logs: ["使用日志", "每次调用的 Token、费用与状态"],
    wallet: ["钱包管理", "余额、冻结与累计消费"],
    channels: ["渠道管理", "配置 OpenAI / DeepSeek 等上游渠道"],
    "models-admin": ["模型管理", "对外模型名、定价并绑定渠道"],
    docs: ["接入说明", "替换 Base URL 即可对接 OpenAI 兼容客户端"],
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
    const label = `${state.user.username}${state.user.is_admin ? " · 管理员" : ""}`;
    $("side-user").textContent = label;
    $("top-user").textContent = label;
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
    if (!state.token) return showLogin();
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

  document.querySelectorAll("[data-page]").forEach((btn) => {
    btn.addEventListener("click", () => {
      state.page = btn.dataset.page;
      render();
    });
  });

  function setActive() {
    document.querySelectorAll("[data-page]").forEach((btn) => {
      btn.classList.toggle("active", btn.dataset.page === state.page);
    });
    const m = meta[state.page] || [state.page, ""];
    pageTitle.textContent = m[0];
    pageSub.textContent = m[1];
  }

  function modelFields(m) {
    return {
      id: m.ID || m.id,
      name: m.ModelName || m.model_name || "",
      display: m.DisplayName || m.display_name || m.ModelName || m.model_name || "",
      mode: (m.BillingMode || m.billing_mode || "TOKEN").toUpperCase(),
      inPrice: Number(m.InputPricePer1K ?? m.input_price_per_1k ?? 0),
      outPrice: Number(m.OutputPricePer1K ?? m.output_price_per_1k ?? 0),
      reqPrice: m.PricePerRequest?.Valid ? Number(m.PricePerRequest.Float64) : (m.price_per_request ?? null),
      maxTokens: m.MaxTokens ?? m.max_tokens ?? 0,
      enabled: m.Enabled ?? m.enabled ?? true,
      stream: m.SupportsStream ?? m.supports_stream ?? true,
    };
  }

  async function loadModels() {
    const data = await api("/api/models");
    state.modelsCache = (data.items || []).map(modelFields);
    return state.modelsCache;
  }

  async function render() {
    setActive();
    pageEl.innerHTML = `<div class="panel"><p class="hint">加载中…</p></div>`;
    try {
      if (state.page === "overview") await renderOverview();
      else if (state.page === "marketplace") await renderMarketplace();
      else if (state.page === "chat") await renderChat();
      else if (state.page === "keys") await renderKeys();
      else if (state.page === "logs") await renderLogs();
      else if (state.page === "wallet") await renderWallet();
      else if (state.page === "channels") await renderChannels();
      else if (state.page === "models-admin") await renderModelsAdmin();
      else if (state.page === "docs") renderDocs();
    } catch (e) {
      pageEl.innerHTML = `<div class="panel"><p class="error">${esc(e.message)}</p></div>`;
    }
  }

  async function renderOverview() {
    await refreshBalance();
    let admin = "";
    if (state.user?.is_admin) {
      const s = await api("/api/admin/overview");
      admin = `
        <div class="card"><div class="label">用户</div><div class="value">${s.users}</div></div>
        <div class="card"><div class="label">令牌</div><div class="value">${s.api_keys}</div></div>
        <div class="card"><div class="label">渠道</div><div class="value">${s.channels}</div></div>
        <div class="card"><div class="label">模型</div><div class="value">${s.models}</div></div>
        <div class="card"><div class="label">调用次数</div><div class="value">${s.request_logs}</div></div>
        <div class="card"><div class="label">平台累计消费</div><div class="value">${Number(s.total_consumed).toFixed(4)}</div></div>`;
    }
    pageEl.innerHTML = `
      <div class="cards">
        <div class="card"><div class="label">当前余额</div><div class="value">¥${Number(state.user.balance).toFixed(4)}</div></div>
        <div class="card"><div class="label">冻结中</div><div class="value">¥${Number(state.user.frozen || 0).toFixed(4)}</div></div>
        <div class="card"><div class="label">累计消费</div><div class="value">¥${Number(state.user.total_consumed || 0).toFixed(4)}</div></div>
        ${admin}
      </div>
      <div class="panel">
        <h3>快速开始（商业路径）</h3>
        <ol class="hint" style="line-height:1.8;padding-left:18px;margin:0">
          <li>管理员在「渠道管理」配置 DeepSeek / OpenAI 等上游（OpenAI 兼容）。</li>
          <li>在「模型管理」发布对外模型名并绑定渠道与价格。</li>
          <li>在「令牌管理」创建业务 Key。</li>
          <li>客户端将 Base URL 改为 <code class="mono">http://&lt;服务器&gt;:3000/v1</code>。</li>
        </ol>
        <div class="actions" style="margin-top:14px">
          <button class="btn primary inline" data-go="marketplace">查看模型广场</button>
          <button class="btn" data-go="chat">打开操练场</button>
        </div>
      </div>`;
    pageEl.querySelectorAll("[data-go]").forEach((b) => {
      b.onclick = () => {
        state.page = b.dataset.go;
        render();
      };
    });
  }

  async function renderMarketplace() {
    const models = await loadModels();
    pageEl.innerHTML = `
      <div class="panel">
        <div class="toolbar">
          <input id="mp-q" placeholder="搜索模型名…" />
          <select id="mp-mode">
            <option value="">全部计费</option>
            <option value="TOKEN">按量 TOKEN</option>
            <option value="REQUEST">按次 REQUEST</option>
          </select>
          <span class="hint" id="mp-count">${models.length} 个可用模型</span>
        </div>
      </div>
      <div id="mp-grid" class="model-grid"></div>`;

    const draw = () => {
      const q = ($("mp-q").value || "").trim().toLowerCase();
      const mode = $("mp-mode").value;
      const list = models.filter((m) => {
        const hit = !q || m.name.toLowerCase().includes(q) || m.display.toLowerCase().includes(q);
        const modeOk = !mode || m.mode === mode;
        return hit && modeOk;
      });
      $("mp-count").textContent = `${list.length} 个可用模型`;
      const grid = $("mp-grid");
      if (!list.length) {
        grid.innerHTML = `<div class="empty">暂无匹配模型。管理员可在「模型管理」发布模型；开发环境可用内置 mock-chat。</div>`;
        return;
      }
      grid.innerHTML = list
        .map((m) => {
          const price =
            m.mode === "REQUEST"
              ? `<div class="price-line">按次：<strong>¥${Number(m.reqPrice || 0).toFixed(4)}</strong> / 次</div>`
              : `<div class="price-line">输入 <strong>¥${m.inPrice.toFixed(4)}</strong> / 1K tokens<br/>输出 <strong>¥${m.outPrice.toFixed(4)}</strong> / 1K tokens</div>`;
          return `<article class="model-card">
            <div class="title">
              <h4>${esc(m.display || m.name)}</h4>
              <span class="badge ${m.enabled ? "on" : "off"}">${m.enabled ? "可用" : "停用"}</span>
            </div>
            <div class="mono hint">${esc(m.name)}</div>
            <div>
              <span class="badge ${m.mode === "REQUEST" ? "request" : "token"}">${esc(m.mode)}</span>
              ${m.stream ? '<span class="badge on" style="margin-left:6px">Stream</span>' : ""}
            </div>
            ${price}
            <div class="foot">
              <span class="hint">上下文 ${m.maxTokens || "—"}</span>
              <button class="btn primary inline" data-use="${esc(m.name)}">去调试</button>
            </div>
          </article>`;
        })
        .join("");
      grid.querySelectorAll("[data-use]").forEach((btn) => {
        btn.onclick = () => {
          state.page = "chat";
          state.preferModel = btn.dataset.use;
          render();
        };
      });
    };
    $("mp-q").oninput = draw;
    $("mp-mode").onchange = draw;
    draw();
  }

  async function renderChat() {
    const models = await loadModels();
    const prefer = state.preferModel || "mock-chat";
    const opts = models
      .map((m) => `<option value="${esc(m.name)}" ${m.name === prefer ? "selected" : ""}>${esc(m.display || m.name)}</option>`)
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <div class="row">
          <div>
            <label>模型</label>
            <select id="chat-model">${opts || '<option value="mock-chat">mock-chat</option>'}</select>
          </div>
        </div>
        <label>Prompt</label>
        <textarea id="chat-input">你好，请用一句话介绍你自己。</textarea>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="chat-send">发送</button>
          <button class="btn" data-go="marketplace">返回模型广场</button>
        </div>
        <label style="margin-top:14px">响应</label>
        <div class="msg" id="chat-out">—</div>
      </div>`;
    pageEl.querySelector("[data-go]").onclick = () => {
      state.page = "marketplace";
      render();
    };
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
        out.textContent = data.choices?.[0]?.message?.content || JSON.stringify(data, null, 2);
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
        <td>${k.id}</td><td>${esc(k.name)}</td><td class="mono">${esc(k.key_prefix)}</td>
        <td>${k.status === 1 ? '<span class="ok">启用</span>' : "停用"}</td>
        <td>${k.rpm_limit} / ${k.tpm_limit}</td><td>${k.quota_used}</td>
        <td><button class="btn danger sm" data-del="${k.id}">删除</button></td></tr>`
      )
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>创建令牌</h3>
        <div class="row">
          <div><label>名称</label><input id="key-name" placeholder="production-app" /></div>
          <div><label>RPM</label><input id="key-rpm" type="number" value="60" /></div>
          <div><label>TPM</label><input id="key-tpm" type="number" value="100000" /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="key-create">创建</button>
        </div>
        <p class="hint" id="key-created">完整密钥仅创建时展示一次，请妥善保存。</p>
      </div>
      <div class="panel">
        <h3>令牌列表</h3>
        <table class="table">
          <thead><tr><th>ID</th><th>名称</th><th>前缀</th><th>状态</th><th>RPM/TPM</th><th>已用</th><th></th></tr></thead>
          <tbody>${rows || '<tr><td colspan="7">暂无令牌</td></tr>'}</tbody>
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
        $("key-created").innerHTML = `请立即复制：<code class="mono">${esc(res.api_key)}</code>`;
        renderKeys();
      } catch (e) {
        $("key-created").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
    pageEl.querySelectorAll("[data-del]").forEach((btn) => {
      btn.onclick = async () => {
        if (!confirm("确认删除？")) return;
        await api("/api/keys?id=" + btn.dataset.del, { method: "DELETE" });
        renderKeys();
      };
    });
  }

  async function renderLogs() {
    const data = await api("/api/logs");
    const rows = (data.items || [])
      .map((l) => {
        const ok = l.Success ?? l.success;
        return `<tr>
          <td class="mono">${esc(l.CreatedAt || l.created_at || "")}</td>
          <td>${esc(l.ModelName || l.model_name || "")}</td>
          <td>${l.InputTokens ?? l.input_tokens ?? 0} / ${l.OutputTokens ?? l.output_tokens ?? 0}</td>
          <td>¥${Number(l.Cost ?? l.cost ?? 0).toFixed(6)}</td>
          <td>${ok ? '<span class="ok">成功</span>' : '<span class="bad">失败</span>'}</td>
          <td>${l.LatencyMs ?? l.latency_ms ?? 0} ms</td>
          <td class="mono">${esc(l.TraceID || l.trace_id || "")}</td>
        </tr>`;
      })
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
        <div class="card"><div class="label">可用余额</div><div class="value">¥${Number(state.user.balance).toFixed(6)}</div></div>
        <div class="card"><div class="label">冻结</div><div class="value">¥${Number(state.user.frozen || 0).toFixed(6)}</div></div>
        <div class="card"><div class="label">累计消费</div><div class="value">¥${Number(state.user.total_consumed || 0).toFixed(6)}</div></div>
      </div>
      <div class="panel"><p class="hint">在线充值 / 兑换码将在后续商业化阶段启用。当前可用预置余额或管理员调账。</p></div>`;
  }

  async function renderChannels() {
    const data = await api("/api/admin/console/channels");
    const rows = (data.items || [])
      .map(
        (c) => `<tr>
        <td>${c.id}</td><td>${esc(c.name)}</td><td>${esc(c.provider)}</td>
        <td class="mono">${esc(c.base_url)}</td><td class="mono">${esc(c.api_key_masked)}</td>
        <td>${c.weight}/${c.priority}</td><td>${c.status === 1 ? "启用" : "停用"}</td></tr>`
      )
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>新增上游渠道</h3>
        <p class="hint">商业主路径请选 openai_compatible，填写 DeepSeek / OpenAI 官方兼容地址。</p>
        <div class="row">
          <div><label>名称</label><input id="ch-name" placeholder="deepseek-main" /></div>
          <div><label>类型</label>
            <select id="ch-provider">
              <option value="openai_compatible">openai_compatible（真实上游）</option>
              <option value="mock">mock（离线联调）</option>
            </select>
          </div>
        </div>
        <div class="row">
          <div><label>Base URL</label><input id="ch-url" placeholder="https://api.deepseek.com/v1" /></div>
          <div><label>上游 API Key</label><input id="ch-key" placeholder="sk-..." /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="ch-save">保存渠道</button>
        </div>
        <p class="hint" id="ch-msg"></p>
      </div>
      <div class="panel">
        <h3>渠道列表</h3>
        <table class="table">
          <thead><tr><th>ID</th><th>名称</th><th>类型</th><th>Base URL</th><th>Key</th><th>权重/优先级</th><th>状态</th></tr></thead>
          <tbody>${rows || '<tr><td colspan="7">暂无渠道</td></tr>'}</tbody>
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
        $("ch-msg").innerHTML = `<span class="ok">已创建渠道 #${res.id}</span>`;
        renderChannels();
      } catch (e) {
        $("ch-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
  }

  async function renderModelsAdmin() {
    const [models, channels] = await Promise.all([
      api("/api/admin/console/models"),
      api("/api/admin/console/channels"),
    ]);
    const chOpts = (channels.items || [])
      .map((c) => `<option value="${c.id}">#${c.id} ${esc(c.name)}</option>`)
      .join("");
    const rows = (models.items || [])
      .map((m) => {
        const f = modelFields(m);
        return `<tr><td>${f.id}</td><td class="mono">${esc(f.name)}</td><td>${esc(f.display)}</td>
          <td>${f.inPrice} / ${f.outPrice}</td><td>${f.enabled ? "启用" : "停用"}</td></tr>`;
      })
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>发布对外模型</h3>
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
          <button class="btn primary inline" id="m-save">发布到模型广场</button>
        </div>
        <p class="hint" id="m-msg"></p>
      </div>
      <div class="panel">
        <h3>已发布模型</h3>
        <table class="table">
          <thead><tr><th>ID</th><th>模型名</th><th>显示名</th><th>In/Out</th><th>状态</th></tr></thead>
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
        $("m-msg").innerHTML = `<span class="ok">已发布 #${res.id}，可在模型广场查看</span>`;
        renderModelsAdmin();
      } catch (e) {
        $("m-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
  }

  function renderDocs() {
    pageEl.innerHTML = `
      <div class="panel">
        <h3>OpenAI 兼容接入</h3>
        <p class="hint">将客户端 Base URL 替换为：</p>
        <p class="mono">http://&lt;服务器IP&gt;:3000/v1</p>
        <p class="hint" style="margin-top:10px">鉴权头：</p>
        <p class="mono">Authorization: Bearer sk-xxxxxxxx</p>
        <p class="hint" style="margin-top:10px">常用路径：</p>
        <ul class="hint">
          <li><code>/v1/models</code></li>
          <li><code>/v1/chat/completions</code></li>
        </ul>
      </div>
      <div class="panel">
        <h3>阶段门禁</h3>
        <p class="hint">每个阶段都执行：Mock 打通 → 商业化验收。详见文档 <code>docs/08-商业化对标与阶段门禁.md</code>。</p>
      </div>`;
  }

  function esc(s) {
    return String(s ?? "")
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
  }

  boot();
})();
