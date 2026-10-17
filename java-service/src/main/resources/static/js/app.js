(() => {
  "use strict";

  const POLL_MS = 2000;
  const VARIANT_SCORES = { A: 0.91, B: 0.86, C: 0.79 };

  const STATUS_LABELS = {
    intake: "приём",
    clarifying: "уточнения",
    skeleton_proposed: "каркас готов",
    skeleton_approved: "каркас утверждён",
    content_generating: "генерация содержания",
    content_ready: "содержание готово",
    validated: "проверено",
    exported: "выгружено",
    failed: "ошибка",
  };

  const state = {
    programs: [],
    currentId: null,
    snapshot: null,
    tab: "request",
    selectedVariant: null,
    pollTimer: null,
    lastBusy: false,
  };

  const el = {
    list: document.getElementById("program-list"),
    view: document.getElementById("view"),
    title: document.getElementById("page-title"),
    status: document.getElementById("status-badge"),
    hours: document.getElementById("hours-badge"),
    working: document.getElementById("working"),
    toasts: document.getElementById("toasts"),
    tabs: document.getElementById("tabs"),
  };

  /* ---------- API ---------- */

  async function api(path, options = {}) {
    const res = await fetch(path, {
      headers: { "Content-Type": "application/json", ...(options.headers || {}) },
      ...options,
    });
    let body = null;
    const text = await res.text();
    if (text) {
      try { body = JSON.parse(text); } catch { body = { message: text }; }
    }
    if (!res.ok) {
      const err = new Error((body && (body.message || body.error)) || res.statusText);
      err.status = res.status;
      err.body = body;
      throw err;
    }
    return body;
  }

  /* ---------- UI helpers ---------- */

  function toast(message, kind = "ok") {
    const node = document.createElement("div");
    node.className = `toast ${kind}`;
    node.textContent = message;
    el.toasts.appendChild(node);
    setTimeout(() => node.remove(), 4200);
  }

  function escapeHtml(s) {
    return String(s ?? "")
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
  }

  function statusClass(status, busy) {
    if (status === "failed") return "failed";
    if (busy || status === "content_generating") return "busy";
    if (status === "validated" || status === "exported") return "ok";
    if (status === "skeleton_proposed" || status === "content_ready") return "info";
    return "";
  }

  async function handleAction(fn) {
    try {
      await fn();
    } catch (err) {
      if (err.status === 409) {
        toast("Сейчас идёт генерация — подождите (409).", "err");
      } else {
        toast(err.message || "Ошибка запроса", "err");
      }
      await refreshCurrent();
      render();
    }
  }

  /* ---------- Data ---------- */

  async function refreshList() {
    state.programs = await api("/programs");
    renderSidebar();
  }

  async function refreshCurrent() {
    if (!state.currentId) {
      state.snapshot = null;
      return;
    }
    state.snapshot = await api(`/programs/${state.currentId}`);
    if (!state.selectedVariant && state.snapshot.selectedVariantKey) {
      state.selectedVariant = state.snapshot.selectedVariantKey;
    }
  }

  function startPolling() {
    stopPolling();
    state.pollTimer = setInterval(async () => {
      if (!state.currentId) return;
      try {
        const prev = state.snapshot;
        const wasBusy = !!(prev && prev.busy);
        await refreshCurrent();
        await refreshList();
        const now = state.snapshot;
        if (wasBusy && now && !now.busy) {
          if (now.status === "failed") {
            toast(`Ошибка: ${now.error || "сбой пайплайна"}`, "err");
          } else {
            toast(`Готово: ${STATUS_LABELS[now.status] || now.status}`, "ok");
          }
        }
        renderChrome();
        renderView();
      } catch (e) {
        /* keep UI alive */
      }
    }, POLL_MS);
  }

  function stopPolling() {
    if (state.pollTimer) clearInterval(state.pollTimer);
    state.pollTimer = null;
  }

  async function selectProgram(id) {
    state.currentId = id;
    state.selectedVariant = null;
    await refreshCurrent();
    await refreshList();
    render();
  }

  /* ---------- Render ---------- */

  function render() {
    renderChrome();
    renderSidebar();
    renderView();
  }

  function renderChrome() {
    const s = state.snapshot;
    document.querySelectorAll(".tab").forEach((t) => {
      t.classList.toggle("active", t.dataset.tab === state.tab);
    });

    const titles = {
      request: "Заявка",
      skeleton: "Каркас",
      content: "Содержание",
      report: "Проверка и отчёт",
      export: "Выгрузка",
    };
    el.title.textContent = titles[state.tab] || "ALTDA";

    if (!s) {
      el.status.textContent = "нет программы";
      el.status.className = "badge";
      el.hours.classList.add("is-hidden");
      el.working.classList.add("is-hidden");
      return;
    }

    el.status.textContent = STATUS_LABELS[s.status] || s.status;
    el.status.className = `badge ${statusClass(s.status, s.busy)}`;
    el.working.classList.toggle("is-hidden", !s.busy);

    const headerHours = s.program?.header?.totalHours;
    const totals = s.program?.curriculum?.totals?.total;
    if (headerHours != null && totals != null) {
      el.hours.classList.remove("is-hidden");
      el.hours.textContent = `${totals}/${headerHours} ч`;
    } else if (headerHours != null) {
      el.hours.classList.remove("is-hidden");
      el.hours.textContent = `заявлено ${headerHours} ч`;
    } else {
      el.hours.classList.add("is-hidden");
    }
  }

  function renderSidebar() {
    if (!state.programs.length) {
      el.list.innerHTML = `<li class="muted" style="padding:0.5rem">Пока нет заявок</li>`;
      return;
    }
    el.list.innerHTML = state.programs.map((p) => `
      <li>
        <button type="button" data-id="${p.id}" class="${p.id === state.currentId ? "active" : ""}">
          <span class="title">${escapeHtml(p.title || "Без названия")}</span>
          <span class="meta">${escapeHtml(STATUS_LABELS[p.status] || p.status)}${p.busy ? " · работает" : ""}</span>
        </button>
      </li>
    `).join("");
  }

  function renderView() {
    const s = state.snapshot;
    if (!s && state.tab !== "request") {
      el.view.innerHTML = `<div class="panel empty">Выберите заявку слева или создайте новую.</div>`;
      return;
    }
    switch (state.tab) {
      case "request":
        el.view.innerHTML = renderRequestTab(s);
        bindRequestTab();
        break;
      case "skeleton":
        el.view.innerHTML = renderSkeletonTab(s);
        bindSkeletonTab();
        break;
      case "content":
        el.view.innerHTML = renderContentTab(s);
        bindContentTab();
        break;
      case "report":
        el.view.innerHTML = renderReportTab(s);
        break;
      case "export":
        el.view.innerHTML = renderExportTab(s);
        bindExportTab();
        break;
      default:
        el.view.innerHTML = "";
    }
  }

  function historyPanel(s) {
    const rows = (s?.history || []).slice().reverse().map((h) => `
      <tr>
        <td>${escapeHtml(new Date(h.at).toLocaleString("ru-RU"))}</td>
        <td>${escapeHtml(h.action)}</td>
        <td>${escapeHtml(h.fromStatus || "")} → ${escapeHtml(h.toStatus || "")}</td>
        <td>${escapeHtml(h.comment || "—")}</td>
      </tr>
    `).join("") || `<tr><td colspan="4" class="muted">История пуста</td></tr>`;

    return `
      <section class="panel">
        <h3>История утверждений и правок</h3>
        <div class="history">
          <table>
            <thead><tr><th>Когда</th><th>Действие</th><th>Статус</th><th>Комментарий</th></tr></thead>
            <tbody>${rows}</tbody>
          </table>
        </div>
      </section>`;
  }

  /* Tab 1 */
  function renderRequestTab(s) {
    if (!s) {
      return `
        <section class="panel stack">
          <h2>Новая заявка</h2>
          <p class="muted">Опишите программу свободным текстом. Агент уточнит недостающие параметры.</p>
          <label class="field">Текст заявки
            <textarea id="raw-text" placeholder="Например: ДПП ПК Анализ данных на Python, 72 ч, очно…">ДПП ПК Анализ данных на Python, 72 ч</textarea>
          </label>
          <div class="row">
            <button type="button" class="btn" id="submit-request">Отправить</button>
          </div>
        </section>`;
    }

    const params = s.program?.request?.params || {};
    const clarifications = s.program?.request?.clarifications || [];
    const banners = [];
    if (s.status === "failed") {
      banners.push(`<div class="banner failed">Сбой: ${escapeHtml(s.error || "неизвестная ошибка")}. Можно ответить на уточнения или вернуть каркас/содержание.</div>`);
    }
    if (s.busy) {
      banners.push(`<div class="banner busy">Идёт обработка — интерфейс опрашивает статус каждые 2 с.</div>`);
    }

    const paramFields = [
      ["programType", "Тип"],
      ["title", "Название"],
      ["domain", "Направление"],
      ["totalHours", "Часы"],
      ["audience", "Аудитория"],
      ["entryLevel", "Входной уровень"],
      ["studyForm", "Форма"],
      ["finalAttestationForm", "Итоговая аттестация"],
    ].map(([k, label]) => `
      <label class="field">${label}
        <input readonly value="${escapeHtml(params[k] ?? "—")}"/>
      </label>`).join("");

    const qBlocks = clarifications.map((c, i) => `
      <label class="field">${escapeHtml(c.question || c.param)}
        <input data-param="${escapeHtml(c.param)}" class="clar-answer" value="${escapeHtml(c.answer ?? "")}" ${s.busy ? "disabled" : ""}/>
      </label>`).join("") || `<p class="muted">Уточняющих вопросов нет.</p>`;

    return `
      ${banners.join("")}
      <section class="panel stack">
        <h2>Исходный запрос</h2>
        <p>${escapeHtml(s.program?.request?.rawText || "")}</p>
      </section>
      <section class="panel stack">
        <h2>Распознанные параметры</h2>
        <p class="muted">Проверьте поля до генерации каркаса.</p>
        <div class="form-grid">${paramFields}</div>
      </section>
      <section class="panel stack">
        <h2>Уточнения</h2>
        <div class="stack" id="clar-form">${qBlocks}</div>
        <div class="row">
          <button type="button" class="btn" id="submit-clar" ${s.busy || s.status === "skeleton_proposed" ? "disabled" : ""}>Ответить и продолжить</button>
        </div>
      </section>
      ${historyPanel(s)}`;
  }

  function bindRequestTab() {
    const submit = document.getElementById("submit-request");
    if (submit) {
      submit.onclick = () => handleAction(async () => {
        const rawText = document.getElementById("raw-text").value.trim();
        if (!rawText) throw new Error("Введите текст заявки");
        const created = await api("/programs", {
          method: "POST",
          body: JSON.stringify({ rawText }),
        });
        toast("Заявка принята", "ok");
        state.currentId = created.id;
        state.tab = "request";
        await refreshList();
        await refreshCurrent();
        render();
      });
    }
    const clar = document.getElementById("submit-clar");
    if (clar) {
      clar.onclick = () => handleAction(async () => {
        const answers = [...document.querySelectorAll(".clar-answer")]
          .map((input) => ({ param: input.dataset.param, answer: input.value.trim() }))
          .filter((a) => a.answer);
        if (!answers.length) throw new Error("Заполните хотя бы один ответ");
        await api(`/programs/${state.currentId}/clarifications`, {
          method: "POST",
          body: JSON.stringify({ answers }),
        });
        toast("Ответы отправлены", "ok");
        await refreshCurrent();
        render();
      });
    }
  }

  /* Tab 2 */
  function renderSkeletonTab(s) {
    if (!s) return "";
    const variantsWrap = s.skeletonVariants?.variants || s.skeletonVariants || [];
    const variants = Array.isArray(variantsWrap) ? variantsWrap : [];
    const canPick = s.status === "skeleton_proposed" && !s.busy;

    if (!variants.length) {
      return `
        <section class="panel empty">
          ${s.busy ? "Каркас генерируется…" : "Каркас ещё не готов. Заполните заявку и уточнения."}
        </section>
        ${historyPanel(s)}`;
    }

    const cards = variants.map((v) => {
      const key = v.key;
      const score = VARIANT_SCORES[key] ?? 0.8;
      const selected = state.selectedVariant === key;
      const modules = (v.modules || []).map((m) =>
        `<li><strong>${escapeHtml(m.id)}</strong> ${escapeHtml(m.title)} <span class="muted">(${(m.topics || []).length} тем)</span></li>`
      ).join("");
      return `
        <article class="variant-card ${selected ? "selected" : ""}" data-key="${escapeHtml(key)}">
          <div class="row" style="justify-content:space-between">
            <strong>Вариант ${escapeHtml(key)}</strong>
            <span class="score">оценка ${score.toFixed(2)}</span>
          </div>
          <p>${escapeHtml(v.rationale || "")}</p>
          <p class="muted"><strong>Цель:</strong> ${escapeHtml(v.goal || "")}</p>
          <ul>${modules}</ul>
          <p class="muted">+ ${escapeHtml((v.pros || []).join("; ") || "—")}</p>
          <p class="muted">− ${escapeHtml((v.cons || []).join("; ") || "—")}</p>
          <button type="button" class="btn btn-secondary btn-sm pick-variant" data-key="${escapeHtml(key)}" ${canPick ? "" : "disabled"}>Выбрать</button>
        </article>`;
    }).join("");

    const lockedBlocks = collectLockableNodes(s.program).map((n) => `
      <div class="list-block">
        <header>
          <span><strong>${escapeHtml(n.id)}</strong> ${escapeHtml(n.title)}</span>
          <span>
            ${n.locked ? `<span class="lock">🔒 залочено</span>` : ""}
            <button type="button" class="btn btn-secondary btn-sm toggle-lock" data-id="${escapeHtml(n.id)}" data-locked="${n.locked}" ${s.busy ? "disabled" : ""}>
              ${n.locked ? "Разблокировать" : "Залочить"}
            </button>
          </span>
        </header>
      </div>`).join("") || `<p class="muted">После утверждения каркаса здесь появятся блоки для блокировки.</p>`;

    return `
      <section class="panel stack">
        <div class="row" style="justify-content:space-between">
          <h2>Три варианта каркаса</h2>
          <span class="badge hours-badge">${escapeHtml(String(s.program?.header?.totalHours ?? s.program?.request?.params?.totalHours ?? "—"))} ч в заявке</span>
        </div>
        <div class="variants">${cards}</div>
        <div class="row">
          <button type="button" class="btn" id="approve-skeleton" ${canPick && state.selectedVariant ? "" : "disabled"}>Утвердить выбранный</button>
          <button type="button" class="btn btn-secondary" id="return-skeleton" ${canPick ? "" : "disabled"}>Вернуть на доработку</button>
        </div>
        <label class="field">Комментарий при возврате
          <textarea id="skeleton-comment" placeholder="Что исправить в каркасе?"></textarea>
        </label>
      </section>
      <section class="panel stack">
        <h3>Блоки: правка / блокировка</h3>
        ${lockedBlocks}
      </section>
      ${historyPanel(s)}`;
  }

  function collectLockableNodes(program) {
    if (!program) return [];
    const out = [];
    (program.competencies || []).forEach((c) => out.push({ id: c.id, title: c.title, locked: !!c._meta?.locked }));
    (program.curriculum?.modules || []).forEach((m) => {
      out.push({ id: m.id, title: m.title, locked: !!m._meta?.locked });
      (m.topics || []).forEach((t) => out.push({ id: t.id, title: t.title, locked: !!t._meta?.locked }));
    });
    return out;
  }

  function bindSkeletonTab() {
    document.querySelectorAll(".pick-variant").forEach((btn) => {
      btn.onclick = () => {
        state.selectedVariant = btn.dataset.key;
        renderView();
        renderChrome();
      };
    });
    const approve = document.getElementById("approve-skeleton");
    if (approve) {
      approve.onclick = () => handleAction(async () => {
        if (!state.selectedVariant) throw new Error("Выберите вариант");
        await api(`/programs/${state.currentId}/skeleton/approve`, {
          method: "POST",
          body: JSON.stringify({ variantKey: state.selectedVariant }),
        });
        toast("Каркас утверждён, запускаем содержание", "ok");
        state.tab = "content";
        await refreshCurrent();
        render();
      });
    }
    const ret = document.getElementById("return-skeleton");
    if (ret) {
      ret.onclick = () => handleAction(async () => {
        const comment = document.getElementById("skeleton-comment").value.trim() || "Нужна доработка каркаса";
        await api(`/programs/${state.currentId}/skeleton/return`, {
          method: "POST",
          body: JSON.stringify({ comment }),
        });
        toast("Каркас возвращён", "ok");
        await refreshCurrent();
        render();
      });
    }
    document.querySelectorAll(".toggle-lock").forEach((btn) => {
      btn.onclick = () => handleAction(async () => {
        const locked = btn.dataset.locked !== "true";
        await api(`/programs/${state.currentId}/lock`, {
          method: "POST",
          body: JSON.stringify({ nodeIds: [btn.dataset.id], locked }),
        });
        await refreshCurrent();
        renderView();
      });
    });
  }

  /* Tab 3 */
  function renderContentTab(s) {
    if (!s) return "";
    const ready = ["content_ready", "validated", "exported", "content_generating"].includes(s.status)
      || s.program?.curriculum;
    if (!ready || s.status === "skeleton_proposed" || s.status === "clarifying" || s.status === "intake") {
      return `<section class="panel empty">Содержание появится после утверждения каркаса.</section>${historyPanel(s)}`;
    }

    const modules = (s.program?.curriculum?.modules || []).map((m) => {
      const topics = (m.topics || []).map((t) => `
        <div class="list-block">
          <header>
            <span>${escapeHtml(t.id)} · ${escapeHtml(t.title)} <span class="muted">${t.hours?.total ?? "?"} ч</span></span>
            <button type="button" class="btn btn-secondary btn-sm regen" data-id="${escapeHtml(t.id)}" ${s.busy ? "disabled" : ""}>Перегенерировать</button>
          </header>
        </div>`).join("");
      return `
        <div class="list-block">
          <header>
            <span><strong>${escapeHtml(m.id)}</strong> ${escapeHtml(m.title)} · ${m.hours?.total ?? "?"} ч</span>
            <button type="button" class="btn btn-secondary btn-sm regen" data-id="${escapeHtml(m.id)}" ${s.busy ? "disabled" : ""}>Модуль заново</button>
          </header>
          ${topics}
        </div>`;
    }).join("") || `<p class="muted">Модулей нет</p>`;

    const assessments = (s.program?.assessment?.tests || []).map((t) =>
      `<li>${escapeHtml(t.id || "")} — ${escapeHtml(t.title || "тест")} (${(t.questions || []).length} вопр.)</li>`
    ).join("") || "<li class=\"muted\">Тестов пока нет</li>";

    const canApprove = (s.status === "content_ready" || s.status === "validated") && !s.busy;

    return `
      <section class="panel stack">
        <div class="row" style="justify-content:space-between">
          <h2>Модули и темы</h2>
          <span class="badge hours-badge">${s.program?.curriculum?.totals?.total ?? "—"}/${s.program?.header?.totalHours ?? "—"} ч</span>
        </div>
        ${modules}
      </section>
      <section class="panel stack">
        <h3>Оценочные средства</h3>
        <ul>${assessments}</ul>
        <button type="button" class="btn btn-secondary btn-sm regen" data-id="assessment" ${s.busy ? "disabled" : ""}>Перегенерировать оценку</button>
      </section>
      <section class="panel stack">
        <h3>Согласование содержания</h3>
        <label class="field">Комментарий
          <textarea id="content-comment" placeholder="Замечания методиста"></textarea>
        </label>
        <div class="row">
          <button type="button" class="btn" id="approve-content" ${canApprove ? "" : "disabled"}>Утвердить содержание</button>
          <button type="button" class="btn btn-secondary" id="return-content" ${s.busy ? "disabled" : ""}>Вернуть на доработку</button>
        </div>
      </section>
      ${historyPanel(s)}`;
  }

  function bindContentTab() {
    document.querySelectorAll(".regen").forEach((btn) => {
      btn.onclick = () => handleAction(async () => {
        await api(`/programs/${state.currentId}/regenerate`, {
          method: "POST",
          body: JSON.stringify({
            scope: "content",
            nodeIds: [btn.dataset.id],
            comment: `Перегенерация ${btn.dataset.id}`,
          }),
        });
        toast(`Перегенерация: ${btn.dataset.id}`, "ok");
        await refreshCurrent();
        render();
      });
    });
    const approve = document.getElementById("approve-content");
    if (approve) {
      approve.onclick = () => handleAction(async () => {
        const comment = document.getElementById("content-comment")?.value || null;
        await api(`/programs/${state.currentId}/content/approve`, {
          method: "POST",
          body: JSON.stringify({ comment }),
        });
        toast("Содержание утверждено", "ok");
        state.tab = "report";
        await refreshCurrent();
        render();
      });
    }
    const ret = document.getElementById("return-content");
    if (ret) {
      ret.onclick = () => handleAction(async () => {
        const comment = document.getElementById("content-comment")?.value.trim() || "Доработать содержание";
        await api(`/programs/${state.currentId}/content/return`, {
          method: "POST",
          body: JSON.stringify({ comment }),
        });
        toast("Содержание возвращено", "ok");
        await refreshCurrent();
        render();
      });
    }
  }

  /* Tab 4 */
  function renderReportTab(s) {
    if (!s) return "";
    const issues = s.issues || [];
    const bySev = { error: [], warning: [], info: [] };
    issues.forEach((i) => {
      const sev = i.severity || "info";
      (bySev[sev] || bySev.info).push(i);
    });

    function block(title, list, cls) {
      if (!list.length) return "";
      return `
        <div class="stack">
          <h3>${title} (${list.length})</h3>
          ${list.map((i) => `
            <div class="issue ${cls}">
              <strong>${escapeHtml(i.ruleId)}</strong> — ${escapeHtml(i.message)}
              <div class="muted">${escapeHtml(i.path || "")}</div>
            </div>`).join("")}
        </div>`;
    }

    const sources = (s.program?.sources || []).map((src) => `
      <tr>
        <td>${escapeHtml(src.id)}</td>
        <td>${escapeHtml(src.citation || "").slice(0, 120)}</td>
        <td>${src.verified ? '<span class="badge ok">подтверждён</span>' : '<span class="badge" style="background:var(--warn-bg);color:var(--warn)">не подтверждён</span>'}</td>
      </tr>`).join("") || `<tr><td colspan="3" class="muted">Источников нет</td></tr>`;

    const steps = (s.program?.generation?.steps || []).map((st) => `
      <tr>
        <td>${escapeHtml(st.step)}</td>
        <td>${escapeHtml(st.model)}</td>
        <td>${st.promptTokens ?? "—"}</td>
        <td>${st.completionTokens ?? "—"}</td>
        <td>${st.durationMs ?? "—"} мс</td>
      </tr>`).join("") || `<tr><td colspan="5" class="muted">Нет данных о генерации</td></tr>`;

    const summary = s.validationReport?.summary;

    return `
      <section class="panel stack">
        <h2>Результаты проверки</h2>
        ${summary ? `<p class="muted">Ошибки: ${summary.errors}, предупреждения: ${summary.warnings}, ок: ${summary.passed}</p>` : `<p class="muted">Отчёт появится после валидации.</p>`}
        ${block("Ошибки", bySev.error, "error")}
        ${block("Предупреждения", bySev.warning, "warning")}
        ${block("Инфо", bySev.info, "info")}
        ${!issues.length && s.status === "validated" ? `<p class="muted">Критических замечаний в списке issues нет (или только предупреждения в отчёте).</p>` : ""}
      </section>
      <section class="panel stack">
        <h3>Источники</h3>
        <div class="history">
          <table>
            <thead><tr><th>ID</th><th>Цитирование</th><th>Статус</th></tr></thead>
            <tbody>${sources}</tbody>
          </table>
        </div>
      </section>
      <section class="panel stack">
        <h3>Токены и время</h3>
        <p class="muted">Оценка стоимости: ${s.program?.generation?.costRub ?? "—"} ₽</p>
        <div class="history">
          <table>
            <thead><tr><th>Шаг</th><th>Модель</th><th>prompt</th><th>completion</th><th>Время</th></tr></thead>
            <tbody>${steps}</tbody>
          </table>
        </div>
      </section>
      ${historyPanel(s)}`;
  }

  /* Tab 5 */
  function renderExportTab(s) {
    if (!s) return "";
    return `
      <section class="panel stack">
        <h2>Выгрузка комплекта</h2>
        <p class="muted">Статус: <strong>${escapeHtml(STATUS_LABELS[s.status] || s.status)}</strong>. JSON и .docx доступны для скачивания.</p>
        <div class="row">
          <button type="button" class="btn" id="dl-json">Скачать JSON</button>
          <button type="button" class="btn btn-secondary" id="dl-docx">Скачать .docx</button>
        </div>
        <pre class="json">${escapeHtml(JSON.stringify(s.program, null, 2).slice(0, 4000))}${JSON.stringify(s.program).length > 4000 ? "\n…" : ""}</pre>
      </section>
      ${historyPanel(s)}`;
  }

  function bindExportTab() {
    const jsonBtn = document.getElementById("dl-json");
    if (jsonBtn) {
      jsonBtn.onclick = () => {
        const blob = new Blob([JSON.stringify(state.snapshot.program, null, 2)], { type: "application/json" });
        const a = document.createElement("a");
        a.href = URL.createObjectURL(blob);
        a.download = `program-${state.currentId}.json`;
        a.click();
        URL.revokeObjectURL(a.href);
        toast("JSON сохранён", "ok");
      };
    }
    const docx = document.getElementById("dl-docx");
    if (docx) {
      docx.onclick = () => {
        window.location.href = `/programs/${state.currentId}/export.docx`;
        toast("Скачивание .docx…", "ok");
      };
    }
  }

  /* ---------- Events ---------- */

  document.getElementById("btn-new").onclick = () => {
    state.currentId = null;
    state.snapshot = null;
    state.tab = "request";
    render();
  };

  el.list.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-id]");
    if (!btn) return;
    selectProgram(btn.dataset.id);
  });

  el.tabs.addEventListener("click", (e) => {
    const tab = e.target.closest(".tab");
    if (!tab) return;
    state.tab = tab.dataset.tab;
    render();
  });

  /* Fix history status display — HistoryEntry serializes enums as strings via Jackson default */
  /* Override: ProgramStatus enums in HistoryEntry are Java enums - need @JsonValue. Check serialization. */

  async function boot() {
    try {
      await refreshList();
      if (state.programs[0]) {
        await selectProgram(state.programs[0].id);
      } else {
        render();
      }
      startPolling();
    } catch (e) {
      toast("Не удалось загрузить список программ", "err");
      render();
    }
  }

  boot();
})();
