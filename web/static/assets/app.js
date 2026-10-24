(() => {
  const state = {
    token: localStorage.getItem("aigate_token") || "",
    user: null,
    page: "overview",
    modelsCache: [],
    modules: null,
    chatId: 0,
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
    chat: ["操练场", "多轮对话会保存在左侧，点开可以接着问"],
    keys: ["令牌管理", "创建业务系统使用的 API Key"],
    logs: ["使用日志", "每次调用的 Token、费用与状态"],
    wallet: ["钱包管理", "余额、冻结与累计消费"],
    billing: ["订购套餐", "SaaS Mock 下单：选套餐 → 模拟支付 → 开通模块"],
    channels: ["渠道管理", "配置 OpenAI / DeepSeek 等上游渠道"],
    "models-admin": ["模型管理", "对外模型名、定价并绑定渠道"],
    "saas-admin": ["组织管理", "子账户与用量（私有化）；或套餐订阅（连我方 Server）"],
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
    applyModuleVisibility();
    refreshBalance();
    render();
  }

  function modOn(id) {
    if (!state.modules) return true;
    return !!state.modules[id];
  }

  function applyModuleVisibility() {
    const map = {
      marketplace: "marketplace",
      chat: "playground",
      wallet: "wallet",
    };
    document.querySelectorAll("[data-page]").forEach((btn) => {
      const page = btn.dataset.page;
      const mid = map[page];
      if (!mid) return;
      btn.style.display = modOn(mid) ? "" : "none";
    });
    // Channels / models: capability-based (admin_channels), not is_admin-only.
    const canChannels = modOn("admin_channels");
    document.querySelectorAll('[data-page="channels"]').forEach((btn) => {
      btn.style.display = canChannels ? "" : "none";
    });
    document.querySelectorAll('[data-page="models-admin"]').forEach((btn) => {
      btn.style.display = canChannels ? "" : "none";
    });
    // Org / vendor admin surface stays root-admin only.
    document.querySelectorAll('[data-page="saas-admin"]').forEach((btn) => {
      btn.style.display = state.user?.is_admin ? "" : "none";
    });
    document.querySelectorAll(".nav-group.admin-only").forEach((el) => {
      const show = state.user?.is_admin || canChannels;
      el.style.display = show ? "" : "none";
      if (!state.user?.is_admin && canChannels) {
        el.textContent = "运维";
      } else if (state.user?.is_admin) {
        el.textContent = "系统管理";
      }
    });
    if (!modOn("wallet")) {
      const pill = $("balance-pill");
      if (pill) pill.style.display = "none";
    } else {
      const pill = $("balance-pill");
      if (pill) pill.style.display = "";
    }
  }

  async function loadModules() {
    try {
      const data = await api("/api/session");
      state.modules = (data.entitlements && data.entitlements.modules) || data.modules || {};
      state.entitlements = data.entitlements || null;
      state.deployMode = data.deploy_mode || "onprem";
      state.adminSurface = data.admin_surface || "org";
      if (data.user) {
        state.user = Object.assign({}, state.user || {}, data.user);
      }
      if (data.product_name) {
        document.querySelectorAll(".product-name").forEach((el) => {
          el.textContent = data.product_name;
        });
      }
      const nav = $("nav-admin-surface");
      if (nav) {
        nav.textContent = state.adminSurface === "vendor" ? "套餐与订阅" : "组织管理";
      }
      meta["saas-admin"] =
        state.adminSurface === "vendor"
          ? ["套餐与订阅", "厂商运营：按付费用户分配模块（服务端强校验）"]
          : ["组织管理", "内部管理员：子账户与用量（非厂商 SaaS 后台）"];
      const billNav = $("nav-billing");
      if (billNav) {
        billNav.style.display = state.deployMode === "saas" ? "" : "none";
      }
    } catch (_) {
      state.modules = null;
    }
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
      await loadModules();
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
      await loadModules();
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
    const pageMod = {
      marketplace: "marketplace",
      chat: "playground",
      wallet: "wallet",
      channels: "admin_channels",
      "models-admin": "admin_channels",
    };
    const need = pageMod[state.page];
    if (need && !modOn(need)) {
      pageEl.innerHTML = `<div class="panel"><p class="error">模块未开通：${esc(need)}。请联系供应商按合同启用，或修改 configs 中 modules。</p></div>`;
      return;
    }
    pageEl.innerHTML = `<div class="panel"><p class="hint">加载中…</p></div>`;
    try {
      if (state.page === "overview") await renderOverview();
      else if (state.page === "marketplace") await renderMarketplace();
      else if (state.page === "chat") await renderChat();
      else if (state.page === "keys") await renderKeys();
      else if (state.page === "logs") await renderLogs();
      else if (state.page === "wallet") await renderWallet();
      else if (state.page === "billing") await renderBilling();
      else if (state.page === "channels") await renderChannels();
      else if (state.page === "models-admin") await renderModelsAdmin();
      else if (state.page === "saas-admin") await renderSaasAdmin();
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
          <input id="mp-q" placeholder="搜索模型名或显示名…" />
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
        return hit && modeOk && m.enabled;
      });
      $("mp-count").textContent = `${list.length} 个可用模型`;
      const grid = $("mp-grid");
      if (!list.length) {
        grid.innerHTML = `
          <div class="empty-card">
            <h3>暂无上架模型</h3>
            <p>模型广场是对外售卖货架。管理员需先配置渠道，再发布模型。</p>
            <div class="actions">
              ${state.user?.is_admin
                ? `<button class="btn primary inline" data-go="channels">去配置渠道</button>
                   <button class="btn inline" data-go="models-admin">去发布模型</button>`
                : `<span class="hint">请联系管理员发布模型；开发环境默认有 mock-chat。</span>`}
            </div>
          </div>`;
        grid.querySelectorAll("[data-go]").forEach((b) => {
          b.onclick = () => { state.page = b.dataset.go; render(); };
        });
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
              <span class="badge on">可用</span>
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

  function chatBubble(m) {
    const role = (m.Role || m.role || "").toLowerCase();
    const text = m.Content || m.content || "";
    const model = m.ModelName || m.model_name || "";
    const who = role === "user" ? "你" : "助手";
    const tag = role === "assistant" && model ? `<div class="chat-model">${esc(model)}</div>` : "";
    return `<div class="chat-bubble ${role === "user" ? "user" : "assistant"}">
      <div class="chat-who">${who}</div>
      ${tag}
      <div class="chat-text">${esc(text)}</div>
    </div>`;
  }

  async function renderChat() {
    const [models, sessions] = await Promise.all([
      loadModels(),
      api("/api/chat/sessions").catch(() => ({ items: [] })),
    ]);
    const prefer = state.preferModel || "mock-chat";
    const opts = models
      .map((m) => `<option value="${esc(m.name)}" ${m.name === prefer ? "selected" : ""}>${esc(m.display || m.name)}</option>`)
      .join("");
    const items = sessions.items || [];
    const side = items.length
      ? items
          .map((c) => {
            const id = c.ID || c.id;
            const title = c.Title || c.title || "新对话";
            const model = c.ModelName || c.model_name || "";
            return `<button type="button" class="chat-item ${id === state.chatId ? "active" : ""}" data-cid="${id}">
              <span class="chat-item-title">${esc(title)}</span>
              <span class="chat-item-model">${esc(model)}</span>
              <span class="chat-item-del" data-delc="${id}" title="删除">×</span>
            </button>`;
          })
          .join("")
      : `<div class="empty-inline">还没有对话</div>`;
    pageEl.innerHTML = `
      <div class="chat-shell">
        <aside class="panel chat-side">
          <button class="btn primary" id="chat-new" type="button">新对话</button>
          <div class="chat-list">${side}</div>
        </aside>
        <section class="panel chat-main">
          <div class="chat-thread" id="chat-thread">
            <div class="chat-empty">从这里开始。发送后会出现在左侧，点开可以接着问。会话中途可以换模型，回复上会标出模型名。</div>
          </div>
          <div class="chat-compose">
            <div class="row">
              <div>
                <label>模型</label>
                <select id="chat-model">${opts || '<option value="mock-chat">mock-chat</option>'}</select>
              </div>
              <div>
                <label>联网</label>
                <label class="hint"><input id="chat-web" type="checkbox" checked /> 搜索后再回答</label>
              </div>
            </div>
            <textarea id="chat-input" placeholder="输入问题，Enter 发送，Shift+Enter 换行"></textarea>
            <div class="actions">
              <button class="btn primary inline" id="chat-send" type="button">发送</button>
            </div>
            <p class="hint" id="chat-msg"></p>
          </div>
        </section>
      </div>`;

    const thread = $("chat-thread");
    const paint = (messages) => {
      if (!messages || !messages.length) {
        thread.innerHTML = `<div class="chat-empty">从这里开始。发送后会出现在左侧，点开可以接着问。会话中途可以换模型，回复上会标出模型名。</div>`;
        return;
      }
      thread.innerHTML = messages.map(chatBubble).join("");
      thread.scrollTop = thread.scrollHeight;
    };
    const openSession = async (id) => {
      state.chatId = id;
      pageEl.querySelectorAll(".chat-item").forEach((el) => {
        el.classList.toggle("active", Number(el.dataset.cid) === id);
      });
      if (!id) {
        paint([]);
        return;
      }
      const data = await api("/api/chat/sessions?id=" + id);
      const conv = data.conversation || {};
      const model = conv.ModelName || conv.model_name;
      if (model && $("chat-model").querySelector(`option[value="${CSS.escape(model)}"]`)) {
        $("chat-model").value = model;
        state.preferModel = model;
      }
      paint(data.messages || []);
    };
    if (state.chatId) {
      try {
        await openSession(state.chatId);
      } catch (_) {
        state.chatId = 0;
        paint([]);
      }
    }
    $("chat-new").onclick = () => {
      state.chatId = 0;
      pageEl.querySelectorAll(".chat-item").forEach((el) => el.classList.remove("active"));
      paint([]);
      $("chat-input").focus();
    };
    pageEl.querySelectorAll("[data-cid]").forEach((btn) => {
      btn.onclick = (ev) => {
        if (ev.target.closest("[data-delc]")) return;
        openSession(Number(btn.dataset.cid)).catch((e) => {
          $("chat-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
        });
      };
    });
    pageEl.querySelectorAll("[data-delc]").forEach((btn) => {
      btn.onclick = async (ev) => {
        ev.stopPropagation();
        const id = Number(btn.dataset.delc);
        await api("/api/chat/sessions?id=" + id, { method: "DELETE" });
        if (state.chatId === id) state.chatId = 0;
        renderChat();
      };
    });
    const send = async () => {
      const text = $("chat-input").value.trim();
      if (!text) return;
      const searching = $("chat-web").checked;
      $("chat-msg").textContent = searching ? "正在联网搜索并生成回答…" : "正在回答…";
      $("chat-send").disabled = true;
      try {
        const data = await api("/api/chat", {
          method: "POST",
          body: JSON.stringify({
            conversation_id: state.chatId || 0,
            content: text,
            model: $("chat-model").value,
            stream: false,
            web_search: searching,
          }),
        });
        state.chatId = data.conversation_id || state.chatId;
        state.preferModel = $("chat-model").value;
        $("chat-input").value = "";
        $("chat-msg").textContent = "";
        await renderChat();
        refreshBalance();
      } catch (e) {
        $("chat-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      } finally {
        if ($("chat-send")) $("chat-send").disabled = false;
      }
    };
    $("chat-send").onclick = send;
    $("chat-input").onkeydown = (ev) => {
      if (ev.key === "Enter" && !ev.shiftKey) {
        ev.preventDefault();
        send();
      }
    };
  }

  function splitCSV(raw) {
    return (raw || "")
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean);
  }

  function fmtExpire(v) {
    if (!v) return "永不过期";
    try {
      return new Date(v).toLocaleString();
    } catch (_) {
      return v;
    }
  }

  async function renderKeys() {
    const [data, models] = await Promise.all([api("/api/keys"), loadModels().catch(() => [])]);
    const modelHints = (models || []).map((m) => m.name).join(", ");
    const rows = (data.items || [])
      .map((k) => {
        const modelsText = (k.allowed_models || []).length ? (k.allowed_models || []).join(", ") : "全部";
        const ips = (k.ip_whitelist || []).length ? (k.ip_whitelist || []).join(", ") : "不限";
        return `<tr>
        <td>${k.id}</td><td>${esc(k.name)}</td><td class="mono">${esc(k.key_prefix)}</td>
        <td>${k.status === 1 ? '<span class="ok">启用</span>' : '<span class="bad">停用</span>'}</td>
        <td>${k.rpm_limit} / ${k.tpm_limit}</td>
        <td class="hint">${esc(modelsText)}</td>
        <td class="hint">${esc(ips)}</td>
        <td class="hint">${esc(fmtExpire(k.expired_at))}</td>
        <td class="nowrap">
          <button class="btn sm" data-toggle="${k.id}" data-status="${k.status === 1 ? 0 : 1}">${k.status === 1 ? "停用" : "启用"}</button>
          <button class="btn danger sm" data-del="${k.id}">删除</button>
        </td></tr>`;
      })
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>创建令牌</h3>
        <p class="hint">完整密钥仅创建时展示一次。可限制模型、IP 与过期时间（对标商业令牌字段）。</p>
        <div class="row">
          <div><label>名称</label><input id="key-name" placeholder="production-app" /></div>
          <div><label>过期时间（可选）</label><input id="key-exp" type="datetime-local" /></div>
        </div>
        <div class="row">
          <div><label>RPM</label><input id="key-rpm" type="number" value="60" /></div>
          <div><label>TPM</label><input id="key-tpm" type="number" value="100000" /></div>
        </div>
        <div class="row">
          <div><label>模型白名单（逗号分隔，空=全部）</label>
            <input id="key-models" placeholder="${esc(modelHints || "deepseek-chat, mock-chat")}" /></div>
          <div><label>IP 白名单（逗号/换行，空=不限）</label>
            <input id="key-ips" placeholder="127.0.0.1, 10.0.0.0/8" /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="key-create">创建令牌</button>
        </div>
        <p class="hint" id="key-created">创建后请立即复制完整 sk- 密钥。</p>
      </div>
      <div class="panel">
        <h3>令牌列表</h3>
        <table class="table">
          <thead><tr><th>ID</th><th>名称</th><th>前缀</th><th>状态</th><th>RPM/TPM</th><th>模型</th><th>IP</th><th>过期</th><th></th></tr></thead>
          <tbody>${rows || '<tr><td colspan="9"><div class="empty-inline">暂无令牌，创建一个给业务系统调用 API。</div></td></tr>'}</tbody>
        </table>
      </div>`;
    $("key-create").onclick = async () => {
      try {
        const expRaw = $("key-exp").value;
        const payload = {
          name: $("key-name").value || "default",
          rpm_limit: Number($("key-rpm").value || 60),
          tpm_limit: Number($("key-tpm").value || 100000),
          allowed_models: splitCSV($("key-models").value),
          ip_whitelist: splitCSV($("key-ips").value),
        };
        if (expRaw) payload.expired_at = new Date(expRaw).toISOString();
        const res = await api("/api/keys", { method: "POST", body: JSON.stringify(payload) });
        state.lastKeyPlain = res.api_key;
        renderKeys();
      } catch (e) {
        $("key-created").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
    if (state.lastKeyPlain) {
      const plain = state.lastKeyPlain;
      state.lastKeyPlain = "";
      $("key-created").innerHTML = `请立即复制：<code class="mono">${esc(plain)}</code>
        <button class="btn sm" id="key-copy">复制</button>`;
      const copyBtn = $("key-copy");
      if (copyBtn) {
        copyBtn.onclick = async () => {
          try {
            await navigator.clipboard.writeText(plain);
            copyBtn.textContent = "已复制";
          } catch (_) {
            copyBtn.textContent = "复制失败";
          }
        };
      }
    }
    pageEl.querySelectorAll("[data-del]").forEach((btn) => {
      btn.onclick = async () => {
        if (!confirm("确认删除该令牌？此操作不可恢复。")) return;
        await api("/api/keys?id=" + btn.dataset.del, { method: "DELETE" });
        renderKeys();
      };
    });
    pageEl.querySelectorAll("[data-toggle]").forEach((btn) => {
      btn.onclick = async () => {
        await api("/api/keys", {
          method: "PATCH",
          body: JSON.stringify({ id: Number(btn.dataset.toggle), status: Number(btn.dataset.status) }),
        });
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
      <div class="panel">
        <h3>兑换码充值</h3>
        <p class="hint">Phase 2 商业化 Mock：用兑换码入账。演示码 <code class="mono">AIGATE-WELCOME</code>（¥10）、<code class="mono">AIGATE-DEMO-100</code>（¥100），每用户每种限兑一次。</p>
        <div class="row">
          <div><label>兑换码</label><input id="redeem-code" placeholder="AIGATE-WELCOME" /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="redeem-btn">立即兑换</button>
        </div>
        <p class="hint" id="redeem-msg"></p>
      </div>
      <div class="panel">
        <h3>在线支付 / 套餐订购</h3>
        <p class="hint">SaaS 模式下请打开侧栏「订购套餐」：Mock 下单 → 模拟支付 → 开通套餐模块。真支付通道下一刀再接。</p>
      </div>`;
    $("redeem-btn").onclick = async () => {
      try {
        const res = await api("/api/wallet/redeem", {
          method: "POST",
          body: JSON.stringify({ code: $("redeem-code").value }),
        });
        $("redeem-msg").innerHTML = `<span class="ok">兑换成功 +¥${Number(res.amount).toFixed(2)}，余额 ¥${Number(res.balance).toFixed(4)}</span>`;
        await refreshBalance();
        state.user.balance = res.balance;
      } catch (e) {
        $("redeem-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
  }

  async function renderBilling() {
    if (state.deployMode !== "saas") {
      pageEl.innerHTML = `<div class="panel"><p class="hint">在线订购仅在 <code>deploy.mode=saas</code> 可用。私有化请用许可证开通模块。</p></div>`;
      return;
    }
    const [cat, orders] = await Promise.all([
      api("/api/billing/catalog"),
      api("/api/billing/orders"),
    ]);
    const cards = (cat.items || [])
      .map(
        (p) => `<div class="card" style="text-align:left">
        <div class="label">${esc(p.name)} <span class="mono">(${esc(p.code)})</span></div>
        <div class="value">¥${Number(p.price).toFixed(0)}<span class="hint"> /月·Mock</span></div>
        <p class="hint">${esc(p.note || "")}</p>
        <button class="btn primary sm" data-buy="${esc(p.code)}">下单并模拟支付</button>
      </div>`
      )
      .join("");
    const rows = (orders.items || [])
      .map(
        (o) => `<tr><td class="mono">${esc(o.order_no)}</td><td>${esc(o.plan_code)}</td>
        <td>¥${Number(o.amount).toFixed(2)}</td><td>${esc(o.status)}</td><td class="mono">${esc(o.created_at)}</td>
        <td>${o.status === "PENDING" ? `<button class="btn sm" data-pay="${esc(o.order_no)}">模拟支付</button>` : "—"}</td></tr>`
      )
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>套餐货架（Mock 支付）</h3>
        <p class="hint">${esc(cat.hint || "")}</p>
        <div class="cards">${cards || "<p class='hint'>暂无可购套餐</p>"}</div>
        <p class="hint" id="billing-msg"></p>
      </div>
      <div class="panel">
        <h3>我的订单</h3>
        <table class="table">
          <thead><tr><th>单号</th><th>套餐</th><th>金额</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>
          <tbody>${rows || "<tr><td colspan='6'>暂无订单</td></tr>"}</tbody>
        </table>
      </div>`;
    const buy = async (code) => {
      try {
        const created = await api("/api/billing/orders", {
          method: "POST",
          body: JSON.stringify({ plan_code: code, payment_method: "mock" }),
        });
        const paid = await api("/api/billing/orders/mock-pay", {
          method: "POST",
          body: JSON.stringify({ order_no: created.order.order_no }),
        });
        $("billing-msg").innerHTML = `<span class="ok">已开通 ${esc(paid.order.plan_code)}（${esc(paid.order.order_no)}）。请重新进入页面刷新权益。</span>`;
        await loadModules();
        applyModuleVisibility();
        await renderBilling();
      } catch (e) {
        $("billing-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
    pageEl.querySelectorAll("[data-buy]").forEach((btn) => {
      btn.onclick = () => buy(btn.dataset.buy);
    });
    pageEl.querySelectorAll("[data-pay]").forEach((btn) => {
      btn.onclick = async () => {
        try {
          await api("/api/billing/orders/mock-pay", {
            method: "POST",
            body: JSON.stringify({ order_no: btn.dataset.pay }),
          });
          await loadModules();
          applyModuleVisibility();
          await renderBilling();
        } catch (e) {
          $("billing-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
        }
      };
    });
  }

  // Commercial vendor presets. risk: true = may need OpenAI-compatible endpoint/proxy.
  const CHANNEL_VENDORS = [
    { id: "deepseek", label: "DeepSeek", base: "https://api.deepseek.com/v1" },
    { id: "openai", label: "OpenAI", base: "https://api.openai.com/v1" },
    { id: "azure", label: "Azure OpenAI", base: "", risk: true },
    { id: "anthropic", label: "Anthropic Claude（需兼容端点）", base: "https://api.anthropic.com/v1", risk: true },
    { id: "gemini", label: "Google Gemini（OpenAI 兼容模式）", base: "https://generativelanguage.googleapis.com/v1beta/openai/" },
    { id: "qwen", label: "通义千问 Qwen", base: "https://dashscope.aliyuncs.com/compatible-mode/v1" },
    { id: "moonshot", label: "月之暗面 Kimi", base: "https://api.moonshot.cn/v1" },
    { id: "zhipu", label: "智谱 GLM", base: "https://open.bigmodel.cn/api/paas/v4" },
    { id: "doubao", label: "豆包 Doubao", base: "https://ark.cn-beijing.volces.com/api/v3" },
    { id: "baichuan", label: "百川 Baichuan", base: "https://api.baichuan-ai.com/v1" },
    { id: "minimax", label: "MiniMax", base: "https://api.minimax.chat/v1", risk: true },
    { id: "hunyuan", label: "腾讯混元", base: "https://api.hunyuan.cloud.tencent.com/v1" },
    { id: "siliconflow", label: "SiliconFlow 硅基流动", base: "https://api.siliconflow.cn/v1" },
    { id: "openrouter", label: "OpenRouter", base: "https://openrouter.ai/api/v1" },
    { id: "groq", label: "Groq", base: "https://api.groq.com/openai/v1" },
    { id: "mistral", label: "Mistral", base: "https://api.mistral.ai/v1" },
    { id: "ollama", label: "Ollama（本地）", base: "http://127.0.0.1:11434/v1" },
    { id: "custom", label: "自定义 OpenAI 兼容", base: "" },
    { id: "mock", label: "Mock（离线联调）", base: "" },
  ];

  function vendorLabel(id) {
    const v = CHANNEL_VENDORS.find((x) => x.id === id);
    return v ? v.label : id;
  }

  async function renderChannels() {
    const data = await api("/api/admin/console/channels");
    const rows = (data.items || [])
      .map(
        (c) => `<tr>
        <td>${c.id}</td><td>${esc(c.name)}</td><td>${esc(vendorLabel(c.provider))}</td>
        <td class="mono">${esc(c.base_url)}</td><td class="mono">${esc(c.api_key_masked)}</td>
        <td>${c.weight}/${c.priority}</td>
        <td>${c.status === 1 ? '<span class="ok">启用</span>' : '<span class="bad">停用</span>'}</td>
        <td class="nowrap">
          <button class="btn sm" data-edit="${c.id}">编辑</button>
          <button class="btn sm" data-test="${c.id}">测试</button>
          <button class="btn sm" data-toggle="${c.id}" data-status="${c.status === 1 ? 0 : 1}">${c.status === 1 ? "停用" : "启用"}</button>
        </td></tr>`
      )
      .join("");
    const vendorOpts = CHANNEL_VENDORS.map(
      (v) => `<option value="${v.id}">${esc(v.label)}</option>`
    ).join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3 id="ch-form-title">新增上游渠道</h3>
        <p class="hint">配错了点列表里的「编辑」改原记录，不用再新建一条。Key 留空表示不更换。保存后点「测试」。</p>
        <div class="row">
          <div><label>名称</label><input id="ch-name" placeholder="例如 deepseek-生产" /></div>
          <div><label>类型（厂商）</label>
            <select id="ch-provider">${vendorOpts}</select>
          </div>
        </div>
        <div class="row">
          <div><label>Base URL</label><input id="ch-url" placeholder="选择厂商后自动填充，可改写" /></div>
          <div><label>上游 API Key</label><input id="ch-key" placeholder="sk-..." /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="ch-save">保存渠道</button>
          <button class="btn" id="ch-cancel" style="display:none">取消编辑</button>
        </div>
        <p class="hint" id="ch-msg"></p>
      </div>
      <div class="panel">
        <h3>渠道列表</h3>
        ${(data.items || []).length ? "" : `<div class="empty-inline">还没有渠道。先选一个流行厂商创建，或用 Mock 完成离线演示。</div>`}
        <table class="table">
          <thead><tr><th>ID</th><th>名称</th><th>类型</th><th>Base URL</th><th>Key</th><th>权重/优先级</th><th>状态</th><th></th></tr></thead>
          <tbody>${rows || '<tr><td colspan="8">暂无渠道</td></tr>'}</tbody>
        </table>
      </div>`;
    const byId = {};
    (data.items || []).forEach((c) => {
      byId[c.id] = c;
    });
    let editingId = 0;
    let skipVendor = false;
    const setChannelMode = (id) => {
      editingId = id || 0;
      const editing = editingId > 0;
      $("ch-form-title").textContent = editing ? "编辑渠道 #" + editingId : "新增上游渠道";
      $("ch-save").textContent = editing ? "保存修改" : "保存渠道";
      $("ch-cancel").style.display = editing ? "" : "none";
      if (!editing && $("ch-provider").value !== "mock") $("ch-key").placeholder = "sk-...";
      if (editing) $("ch-key").placeholder = "留空则保持原密钥";
    };
    const applyVendor = () => {
      const id = $("ch-provider").value;
      const v = CHANNEL_VENDORS.find((x) => x.id === id);
      if (!v) return;
      $("ch-url").value = v.base;
      if (!$("ch-name").value.trim()) {
        $("ch-name").placeholder = v.id + "-main";
      }
      if (id === "mock") {
        if (!editingId) $("ch-key").value = "";
        $("ch-key").placeholder = "Mock 无需 Key";
      } else if (id === "azure") {
        $("ch-url").placeholder = "https://YOUR_RESOURCE.openai.azure.com/openai/deployments/...";
        $("ch-key").placeholder = editingId ? "留空则保持原密钥" : "sk-...";
      } else {
        $("ch-key").placeholder = editingId ? "留空则保持原密钥" : "sk-...";
      }
    };
    $("ch-provider").onchange = () => {
      if (!skipVendor) applyVendor();
    };
    applyVendor();
    const riskNote = document.createElement("p");
    riskNote.className = "hint";
    riskNote.id = "ch-risk";
    $("ch-save").parentElement.before(riskNote);
    const syncRisk = () => {
      const v = CHANNEL_VENDORS.find((x) => x.id === $("ch-provider").value);
      $("ch-risk").textContent = v?.risk
        ? "注意：该厂商可能需要官方 OpenAI 兼容端点或代理；原生协议暂不支持。"
        : "网关按 OpenAI 兼容 HTTP 调用上游。";
    };
    $("ch-provider").addEventListener("change", syncRisk);
    syncRisk();
    $("ch-save").onclick = async () => {
      try {
        const provider = $("ch-provider").value;
        if (provider !== "mock" && !$("ch-url").value.trim()) {
          throw new Error("请填写 Base URL（Azure / 自定义类型需手动填写）");
        }
        if (!editingId && provider !== "mock" && !$("ch-key").value.trim()) {
          throw new Error("请填写上游 API Key");
        }
        if (editingId) {
          const payload = {
            id: editingId,
            name: $("ch-name").value.trim() || provider + "-main",
            provider,
            base_url: $("ch-url").value.trim(),
          };
          if ($("ch-key").value.trim()) payload.api_key = $("ch-key").value.trim();
          await api("/api/admin/console/channels", { method: "PATCH", body: JSON.stringify(payload) });
          $("ch-msg").innerHTML = `<span class="ok">已更新渠道 #${editingId}</span>`;
        } else {
          const res = await api("/api/admin/console/channels", {
            method: "POST",
            body: JSON.stringify({
              name: $("ch-name").value || provider + "-main",
              provider,
              base_url: $("ch-url").value,
              api_key: $("ch-key").value,
              weight: 100,
              priority: 10,
            }),
          });
          $("ch-msg").innerHTML = `<span class="ok">已创建渠道 #${res.id}，建议立刻点「测试」</span>`;
        }
        renderChannels();
      } catch (e) {
        $("ch-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
    $("ch-cancel").onclick = () => {
      $("ch-name").value = "";
      $("ch-key").value = "";
      $("ch-msg").textContent = "";
      setChannelMode(0);
      applyVendor();
    };
    pageEl.querySelectorAll("[data-edit]").forEach((btn) => {
      btn.onclick = () => {
        const c = byId[btn.dataset.edit];
        if (!c) return;
        skipVendor = true;
        $("ch-provider").value = c.provider || "custom";
        skipVendor = false;
        $("ch-name").value = c.name || "";
        $("ch-url").value = c.base_url || "";
        $("ch-key").value = "";
        setChannelMode(c.id);
        syncRisk();
        $("ch-msg").textContent = "正在编辑渠道 #" + c.id + "，密钥留空则保持不变。";
      };
    });
    pageEl.querySelectorAll("[data-toggle]").forEach((btn) => {
      btn.onclick = async () => {
        await api("/api/admin/console/channels", {
          method: "PATCH",
          body: JSON.stringify({ id: Number(btn.dataset.toggle), status: Number(btn.dataset.status) }),
        });
        renderChannels();
      };
    });
    pageEl.querySelectorAll("[data-test]").forEach((btn) => {
      btn.onclick = async () => {
        $("ch-msg").textContent = "正在测试连通…";
        try {
          const res = await api("/api/admin/console/channels/test", {
            method: "POST",
            body: JSON.stringify({ id: Number(btn.dataset.test) }),
          });
          $("ch-msg").innerHTML = `<span class="ok">连通成功（${esc(res.model || "上游模型")}）：${esc(res.preview || "ok")}</span>`;
        } catch (e) {
          $("ch-msg").innerHTML = `<span class="bad">连通失败：${esc(e.message)}</span>`;
        }
      };
    });
  }

  async function renderModelsAdmin() {
    const [models, channels] = await Promise.all([
      api("/api/admin/console/models"),
      api("/api/admin/console/channels"),
    ]);
    const chOpts = (channels.items || [])
      .map((c) => `<option value="${c.id}">#${c.id} ${esc(c.name)} (${esc(vendorLabel(c.provider))})</option>`)
      .join("");
    const byId = {};
    (models.items || []).forEach((m) => {
      byId[m.ID || m.id] = m;
    });
    const rows = (models.items || [])
      .map((m) => {
        const f = modelFields(m);
        const up = m.upstream_model || "—";
        const ch = m.channel_name ? `#${m.channel_id} ${m.channel_name}` : "未绑定";
        return `<tr><td>${f.id}</td><td class="mono">${esc(f.name || "（未命名）")}</td><td>${esc(f.display)}</td>
          <td class="mono">${esc(up)}</td><td>${esc(ch)}</td>
          <td>${f.inPrice} / ${f.outPrice}</td>
          <td>${f.enabled ? '<span class="ok">上架</span>' : '<span class="bad">下架</span>'}</td>
          <td class="nowrap">
            <button class="btn sm" data-medit="${f.id}">编辑</button>
            <button class="btn sm" data-en="${f.id}" data-on="${f.enabled ? 0 : 1}">${f.enabled ? "下架" : "上架"}</button>
          </td></tr>`;
      })
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3 id="m-form-title">发布对外模型</h3>
        <p class="hint">对外模型名会出现在模型广场与 /v1/models。配错了点「编辑」改这一条，名称、上游模型、渠道和价格都会更新，不用再发布一条重复的。</p>
        ${!(channels.items || []).length ? `<div class="empty-inline">暂无渠道 — <button class="btn sm" data-go="channels">去创建渠道</button></div>` : ""}
        <div class="row">
          <div><label>对外模型名</label><input id="m-name" placeholder="deepseek-flash" /></div>
          <div><label>显示名</label><input id="m-disp" placeholder="DeepSeek Flash" /></div>
        </div>
        <div class="row">
          <div><label>上游模型名</label><input id="m-up" placeholder="deepseek-flash 或 deepseek-v4-pro" /></div>
          <div><label>绑定渠道</label><select id="m-ch">${chOpts || "<option value=''>请先创建渠道</option>"}</select></div>
        </div>
        <div class="row">
          <div><label>输入价 / 1K</label><input id="m-in" type="number" step="0.0001" value="0.001" /></div>
          <div><label>输出价 / 1K</label><input id="m-out" type="number" step="0.0001" value="0.002" /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="m-save" ${!(channels.items || []).length ? "disabled" : ""}>发布到模型广场</button>
          <button class="btn" id="m-cancel" style="display:none">取消编辑</button>
        </div>
        <p class="hint" id="m-msg"></p>
      </div>
      <div class="panel">
        <h3>已发布模型</h3>
        <table class="table">
          <thead><tr><th>ID</th><th>模型名</th><th>显示名</th><th>上游模型</th><th>渠道</th><th>In/Out</th><th>状态</th><th></th></tr></thead>
          <tbody>${rows || '<tr><td colspan="8"><div class="empty-inline">尚未发布模型。发布后会出现在模型广场。</div></td></tr>'}</tbody>
        </table>
      </div>`;
    pageEl.querySelectorAll("[data-go]").forEach((b) => {
      b.onclick = () => { state.page = b.dataset.go; render(); };
    });
    let editingModelId = 0;
    const resetModelForm = () => {
      editingModelId = 0;
      $("m-form-title").textContent = "发布对外模型";
      $("m-save").textContent = "发布到模型广场";
      $("m-cancel").style.display = "none";
      $("m-name").value = "";
      $("m-disp").value = "";
      $("m-up").value = "";
      $("m-in").value = "0.001";
      $("m-out").value = "0.002";
    };
    $("m-cancel").onclick = () => {
      resetModelForm();
      $("m-msg").textContent = "";
    };
    pageEl.querySelectorAll("[data-medit]").forEach((btn) => {
      btn.onclick = () => {
        const m = byId[btn.dataset.medit];
        if (!m) return;
        const f = modelFields(m);
        editingModelId = f.id;
        $("m-form-title").textContent = "编辑模型 #" + f.id;
        $("m-save").textContent = "保存修改";
        $("m-cancel").style.display = "";
        $("m-name").value = f.name || "";
        $("m-disp").value = f.display || "";
        $("m-up").value = m.upstream_model || "";
        if (m.channel_id) $("m-ch").value = String(m.channel_id);
        $("m-in").value = String(f.inPrice);
        $("m-out").value = String(f.outPrice);
        $("m-msg").textContent = "正在编辑 #" + f.id + "。保存后仍是这一条，广场和操练场会用新配置。";
        $("m-name").focus();
      };
    });
    $("m-save").onclick = async () => {
      try {
        if (!$("m-ch").value) throw new Error("请先创建并选择渠道");
        if (!$("m-name").value.trim()) throw new Error("请填写对外模型名，例如 deepseek-flash");
        if (!$("m-up").value.trim()) throw new Error("请填写上游模型名，例如 deepseek-flash");
        const payload = {
          model_name: $("m-name").value.trim(),
          display_name: $("m-disp").value.trim() || $("m-name").value.trim(),
          upstream_model: $("m-up").value.trim(),
          channel_id: Number($("m-ch").value),
          input_price_per_1k: Number($("m-in").value),
          output_price_per_1k: Number($("m-out").value),
          group_id: 1,
        };
        if (editingModelId) {
          payload.id = editingModelId;
          await api("/api/admin/console/models", { method: "PATCH", body: JSON.stringify(payload) });
          $("m-msg").innerHTML = `<span class="ok">已更新模型 #${editingModelId}</span>`;
        } else {
          const res = await api("/api/admin/console/models", { method: "POST", body: JSON.stringify(payload) });
          $("m-msg").innerHTML = `<span class="ok">已发布 #${res.id}，可在模型广场查看</span>`;
        }
        renderModelsAdmin();
      } catch (e) {
        $("m-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
    pageEl.querySelectorAll("[data-en]").forEach((btn) => {
      btn.onclick = async () => {
        await api("/api/admin/console/models", {
          method: "PATCH",
          body: JSON.stringify({ id: Number(btn.dataset.en), enabled: btn.dataset.on === "1" }),
        });
        renderModelsAdmin();
      };
    });
  }

  async function renderSaasAdmin() {
    if (!state.user?.is_admin) {
      pageEl.innerHTML = `<div class="panel"><p class="error">仅管理员可访问</p></div>`;
      return;
    }
    if (state.adminSurface !== "vendor") {
      return renderOrgAdmin();
    }
    const [plansRes, usersRes, billingRes] = await Promise.all([
      api("/api/admin/plans"),
      api("/api/admin/users"),
      api("/api/admin/billing/orders").catch(() => ({ items: [] })),
    ]);
    const catalog = plansRes.catalog || [];
    const planOpts = (plansRes.items || [])
      .map((p) => `<option value="${p.id}">${esc(p.code)} · ${esc(p.name)}</option>`)
      .join("");
    const planRows = (plansRes.items || [])
      .map((p) => {
        const on = Object.keys(p.modules || {})
          .filter((k) => p.modules[k] && k !== "core" && k !== "console")
          .join(", ");
        return `<tr>
          <td>${p.id}</td><td class="mono">${esc(p.code)}</td><td>${esc(p.name)}</td>
          <td class="hint">${esc(on || "—")}</td>
          <td>${p.max_keys || "不限"}</td>
          <td>${p.status === 1 ? '<span class="ok">启用</span>' : "停用"}</td>
        </tr>`;
      })
      .join("");
    const orderRows = (billingRes.items || [])
      .map(
        (o) => `<tr><td class="mono">${esc(o.order_no)}</td><td>${o.user_id}</td><td>${esc(o.plan_code)}</td>
        <td>¥${Number(o.amount).toFixed(2)}</td><td>${esc(o.status)}</td><td class="mono">${esc(o.created_at)}</td></tr>`
      )
      .join("");
    const userRows = (usersRes.items || [])
      .map(
        (u) => `<tr>
        <td>${u.id}</td><td>${esc(u.username)}</td><td>${esc(u.role)}</td>
        <td>${esc(u.plan_name || u.plan_code || "—")}</td>
        <td>${u.status === 1 ? '<span class="ok">正常</span>' : '<span class="bad">停用</span>'}</td>
        <td>¥${Number(u.balance).toFixed(2)}</td>
        <td class="nowrap">
          <select data-plan-user="${u.id}">${(plansRes.items || [])
            .map((p) => `<option value="${p.id}" ${String(u.plan_id) === String(p.id) ? "selected" : ""}>${esc(p.code)}</option>`)
            .join("")}</select>
          <button class="btn sm" data-save-plan="${u.id}">保存套餐</button>
          <button class="btn sm" data-toggle-user="${u.id}" data-status="${u.status === 1 ? 0 : 1}">${u.status === 1 ? "停用" : "启用"}</button>
        </td>
      </tr>`
      )
      .join("");
    const checks = catalog
      .map((id) => `<label class="mod-check"><input type="checkbox" data-mod="${esc(id)}" /> ${esc(id)}</label>`)
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>厂商运营台（saas）</h3>
        <p class="hint">网络客户端连我方 Server：套餐分配 + 服务端强校验。改 Electron 无法越权。用户侧「订购套餐」走 Mock 支付开通。</p>
      </div>
      <div class="panel">
        <h3>订购订单（Mock）</h3>
        <table class="table">
          <thead><tr><th>单号</th><th>用户</th><th>套餐</th><th>金额</th><th>状态</th><th>时间</th></tr></thead>
          <tbody>${orderRows || "<tr><td colspan='6'>暂无订单</td></tr>"}</tbody>
        </table>
      </div>
      <div class="panel">
        <h3>套餐（SKU）</h3>
        <div class="row">
          <div><label>编码</label><input id="plan-code" placeholder="pro-custom" /></div>
          <div><label>名称</label><input id="plan-name" placeholder="定制专业版" /></div>
        </div>
        <div class="mod-grid">${checks}</div>
        <div class="row" style="margin-top:10px">
          <div><label>最大令牌数（0=不限）</label><input id="plan-keys" type="number" value="20" /></div>
          <div><label>备注</label><input id="plan-note" placeholder="合同备注" /></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="plan-save">保存套餐</button>
        </div>
        <p class="hint" id="plan-msg"></p>
        <table class="table" style="margin-top:14px">
          <thead><tr><th>ID</th><th>编码</th><th>名称</th><th>模块</th><th>令牌上限</th><th>状态</th></tr></thead>
          <tbody>${planRows}</tbody>
        </table>
      </div>
      <div class="panel">
        <h3>付费用户</h3>
        <div class="row">
          <div><label>用户名</label><input id="nu-user" /></div>
          <div><label>密码</label><input id="nu-pass" type="password" /></div>
        </div>
        <div class="row">
          <div><label>套餐</label><select id="nu-plan">${planOpts}</select></div>
          <div><label>角色</label><select id="nu-role"><option value="user">user</option><option value="admin">admin</option></select></div>
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="nu-save">创建用户</button>
        </div>
        <p class="hint" id="nu-msg"></p>
        <table class="table" style="margin-top:14px">
          <thead><tr><th>ID</th><th>用户</th><th>角色</th><th>套餐</th><th>状态</th><th>余额</th><th>操作</th></tr></thead>
          <tbody>${userRows}</tbody>
        </table>
      </div>`;
    $("plan-save").onclick = async () => {
      try {
        const mods = { core: true, console: true };
        pageEl.querySelectorAll("[data-mod]").forEach((el) => {
          mods[el.dataset.mod] = !!el.checked;
        });
        await api("/api/admin/plans", {
          method: "POST",
          body: JSON.stringify({
            code: $("plan-code").value,
            name: $("plan-name").value,
            note: $("plan-note").value,
            max_keys: Number($("plan-keys").value || 0),
            modules: mods,
            status: 1,
          }),
        });
        $("plan-msg").innerHTML = `<span class="ok">套餐已保存</span>`;
        renderSaasAdmin();
      } catch (e) {
        $("plan-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
    $("nu-save").onclick = async () => {
      try {
        await api("/api/admin/users", {
          method: "POST",
          body: JSON.stringify({
            username: $("nu-user").value,
            password: $("nu-pass").value,
            plan_id: Number($("nu-plan").value),
            role: $("nu-role").value,
            balance: 0,
          }),
        });
        $("nu-msg").innerHTML = `<span class="ok">用户已创建</span>`;
        renderSaasAdmin();
      } catch (e) {
        $("nu-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };
    pageEl.querySelectorAll("[data-save-plan]").forEach((btn) => {
      btn.onclick = async () => {
        const id = Number(btn.dataset.savePlan);
        const sel = pageEl.querySelector(`[data-plan-user="${id}"]`);
        await api("/api/admin/users", {
          method: "PATCH",
          body: JSON.stringify({ id, plan_id: Number(sel.value) }),
        });
        renderSaasAdmin();
      };
    });
    pageEl.querySelectorAll("[data-toggle-user]").forEach((btn) => {
      btn.onclick = async () => {
        await api("/api/admin/users", {
          method: "PATCH",
          body: JSON.stringify({ id: Number(btn.dataset.toggleUser), status: Number(btn.dataset.status) }),
        });
        renderSaasAdmin();
      };
    });
  }

  async function renderOrgAdmin() {
    const [usersRes, usageRes, rolesRes] = await Promise.all([
      api("/api/admin/users"),
      api("/api/org/usage"),
      api("/api/org/roles"),
    ]);
    const roles = rolesRes.roles || usersRes.roles || [];
    const assignable = rolesRes.assignable_modules || usersRes.assignable_modules || [];
    const labels = rolesRes.module_labels || {
      marketplace: "模型广场",
      playground: "操练场",
      wallet: "钱包",
      admin_channels: "渠道与模型管理",
      audit: "审计",
    };
    const ceiling = rolesRes.ceiling || {};
    const roleOpts = roles
      .map((r) => `<option value="${esc(r.code)}">${esc(r.name)} — ${esc(r.description)}</option>`)
      .join("");
    const permChecks = (prefix, selected) =>
      assignable
        .map((id) => {
          const disabled = ceiling[id] === false ? "disabled" : "";
          const checked = selected && selected[id] ? "checked" : "";
          const tip = ceiling[id] === false ? "（实例未开通）" : "";
          return `<label class="mod-check"><input type="checkbox" data-perm="${esc(id)}" data-prefix="${esc(prefix)}" ${checked} ${disabled} /> ${esc(labels[id] || id)}${tip}</label>`;
        })
        .join("");
    const summarize = (u) => {
      const eff = u.effective_modules || {};
      const on = assignable.filter((id) => eff[id]).map((id) => labels[id] || id);
      return on.length ? on.join("、") : "仅基础控制台";
    };
    const userRows = (usersRes.items || [])
      .map((u) => {
        const role = u.org_role || "member";
        return `<tr>
        <td>${u.id}</td>
        <td>${esc(u.username)}${u.is_self ? "（本账号）" : ""}</td>
        <td>${esc(u.display_name || "—")}</td>
        <td class="mono">${esc(role)}</td>
        <td class="hint">${esc(summarize(u))}</td>
        <td>${u.status === 1 ? '<span class="ok">正常</span>' : '<span class="bad">停用</span>'}</td>
        <td>¥${Number(u.total_consumed || 0).toFixed(4)}</td>
        <td class="nowrap">
          ${
            u.is_self
              ? "—"
              : `<button class="btn sm" data-edit-user="${u.id}">权限</button>
                 <button class="btn sm" data-toggle-user="${u.id}" data-status="${u.status === 1 ? 0 : 1}">${u.status === 1 ? "停用" : "启用"}</button>`
          }
        </td>
      </tr>`;
      })
      .join("");
    const usageRows = (usageRes.items || [])
      .map(
        (u) => `<tr>
        <td>${esc(u.username)}</td><td>${u.calls}</td>
        <td>${u.input_tokens} / ${u.output_tokens}</td>
        <td>¥${Number(u.cost).toFixed(6)}</td>
        <td class="hint">${esc(u.last_at || "—")}</td>
      </tr>`
      )
      .join("");
    pageEl.innerHTML = `
      <div class="panel">
        <h3>组织管理（私有化 RBAC）</h3>
        <p class="hint">这里管理<strong>本组织成员权限</strong>，不是厂商 SaaS 套餐台。权限 = 实例许可证天花板 ∩ 组织角色/勾选。令牌与日志对所有成员开放；「渠道与模型管理」需运维角色或自定义勾选。</p>
      </div>
      <div class="panel">
        <h3>创建子账户</h3>
        <div class="row">
          <div><label>用户名</label><input id="nu-user" placeholder="dept-a" /></div>
          <div><label>密码</label><input id="nu-pass" type="password" /></div>
        </div>
        <div class="row">
          <div><label>显示名</label><input id="nu-disp" placeholder="研发一组" /></div>
          <div><label>初始余额</label><input id="nu-bal" type="number" step="0.01" value="0" /></div>
        </div>
        <div class="row">
          <div style="flex:1"><label>组织角色</label><select id="nu-role">${roleOpts}</select></div>
        </div>
        <div id="nu-perms-wrap" class="mod-grid" style="margin-top:10px;display:none">
          <p class="hint" style="grid-column:1/-1;margin:0">自定义权限（仅 custom 角色必填；其他角色可作覆盖）</p>
          ${permChecks("nu", {})}
        </div>
        <div class="actions" style="margin-top:12px">
          <button class="btn primary inline" id="nu-save">创建子账户</button>
        </div>
        <p class="hint" id="nu-msg"></p>
      </div>
      <div class="panel">
        <h3>成员与权限</h3>
        <table class="table">
          <thead><tr><th>ID</th><th>账号</th><th>显示名</th><th>角色</th><th>有效权限</th><th>状态</th><th>累计消费</th><th></th></tr></thead>
          <tbody>${userRows || '<tr><td colspan="8">暂无</td></tr>'}</tbody>
        </table>
        <div id="edit-panel" class="panel" style="display:none;margin-top:14px;background:#f9fafb">
          <h3>编辑成员权限</h3>
          <input type="hidden" id="ed-id" />
          <div class="row">
            <div><label>显示名</label><input id="ed-disp" /></div>
            <div><label>重置密码（可选）</label><input id="ed-pass" type="password" placeholder="留空不改" /></div>
          </div>
          <div class="row">
            <div style="flex:1"><label>组织角色</label><select id="ed-role">${roleOpts}</select></div>
          </div>
          <div id="ed-perms-wrap" class="mod-grid" style="margin-top:10px">${permChecks("ed", {})}</div>
          <div class="actions" style="margin-top:12px">
            <button class="btn primary inline" id="ed-save">保存权限</button>
            <button class="btn sm" id="ed-cancel">取消</button>
          </div>
          <p class="hint" id="ed-msg"></p>
        </div>
      </div>
      <div class="panel">
        <h3>子账户用量</h3>
        <table class="table">
          <thead><tr><th>账号</th><th>调用次数</th><th>In/Out tokens</th><th>费用</th><th>最近调用</th></tr></thead>
          <tbody>${usageRows || '<tr><td colspan="5">暂无调用</td></tr>'}</tbody>
        </table>
      </div>`;

    const readPerms = (prefix) => {
      const perms = {};
      pageEl.querySelectorAll(`[data-prefix="${prefix}"][data-perm]`).forEach((el) => {
        perms[el.dataset.perm] = !!el.checked;
      });
      return perms;
    };
    const toggleCustom = (selId, wrapId) => {
      const role = $(selId).value;
      $(wrapId).style.display = role === "custom" ? "" : "none";
    };
    $("nu-role").onchange = () => toggleCustom("nu-role", "nu-perms-wrap");
    toggleCustom("nu-role", "nu-perms-wrap");

    $("nu-save").onclick = async () => {
      try {
        const org_role = $("nu-role").value;
        const payload = {
          username: $("nu-user").value,
          password: $("nu-pass").value,
          display_name: $("nu-disp").value,
          balance: Number($("nu-bal").value || 0),
          org_role,
        };
        if (org_role === "custom") payload.perms = readPerms("nu");
        await api("/api/admin/users", { method: "POST", body: JSON.stringify(payload) });
        $("nu-msg").innerHTML = `<span class="ok">子账户已创建</span>`;
        renderOrgAdmin();
      } catch (e) {
        $("nu-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
      }
    };

    const byId = {};
    (usersRes.items || []).forEach((u) => {
      byId[u.id] = u;
    });
    pageEl.querySelectorAll("[data-toggle-user]").forEach((btn) => {
      btn.onclick = async () => {
        await api("/api/admin/users", {
          method: "PATCH",
          body: JSON.stringify({ id: Number(btn.dataset.toggleUser), status: Number(btn.dataset.status) }),
        });
        renderOrgAdmin();
      };
    });
    pageEl.querySelectorAll("[data-edit-user]").forEach((btn) => {
      btn.onclick = () => {
        const u = byId[Number(btn.dataset.editUser)];
        if (!u) return;
        $("edit-panel").style.display = "";
        $("ed-id").value = String(u.id);
        $("ed-disp").value = u.display_name || "";
        $("ed-pass").value = "";
        $("ed-role").value = u.org_role || "member";
        const preset = (roles.find((r) => r.code === $("ed-role").value) || {}).modules || {};
        const selected = Object.assign({}, preset, u.perms || {});
        pageEl.querySelectorAll('[data-prefix="ed"][data-perm]').forEach((el) => {
          el.checked = !!selected[el.dataset.perm];
        });
        $("ed-perms-wrap").style.display = "";
        $("ed-msg").textContent = "";
        $("edit-panel").scrollIntoView({ behavior: "smooth", block: "nearest" });
      };
    });
    $("ed-role").onchange = () => {
      const preset = (roles.find((r) => r.code === $("ed-role").value) || {}).modules || {};
      pageEl.querySelectorAll('[data-prefix="ed"][data-perm]').forEach((el) => {
        if ($("ed-role").value === "custom") {
          /* keep current checks */
        } else {
          el.checked = !!preset[el.dataset.perm];
        }
      });
    };
    $("ed-cancel").onclick = () => {
      $("edit-panel").style.display = "none";
    };
    $("ed-save").onclick = async () => {
      try {
        const id = Number($("ed-id").value);
        const org_role = $("ed-role").value;
        const payload = {
          id,
          display_name: $("ed-disp").value,
          org_role,
          perms: readPerms("ed"),
        };
        if ($("ed-pass").value) payload.password = $("ed-pass").value;
        await api("/api/admin/users", { method: "PATCH", body: JSON.stringify(payload) });
        $("ed-msg").innerHTML = `<span class="ok">已保存</span>`;
        renderOrgAdmin();
      } catch (e) {
        $("ed-msg").innerHTML = `<span class="bad">${esc(e.message)}</span>`;
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
        <h3>推荐配置顺序</h3>
        <ol class="hint" style="line-height:1.8;padding-left:18px;margin:0">
          <li>渠道管理：选厂商 → 填 Key → 测试连通</li>
          <li>模型管理：发布对外模型并定价</li>
          <li>令牌管理：创建业务 Key（可设过期/模型/IP）</li>
          <li>钱包：兑换码充值（演示码 AIGATE-WELCOME）</li>
          <li>操练场或外部 SDK 调用验证</li>
        </ol>
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
