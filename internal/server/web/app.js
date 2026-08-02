// Glue only: the server owns the markdown, the offsets and the file.

const doc = document.getElementById("doc");
const composer = document.getElementById("composer");
const threadList = document.getElementById("threads");
const toast = document.getElementById("toast");
const selectionMenu = document.getElementById("selection-menu");
const handoffPanel = document.getElementById("handoff");

// fields comes from the server, so the controls here and the payload the server builds
// can never describe different schemas.
const state = { rev: null, threads: [], fields: [], pending: null, selected: null };

const encoder = new TextEncoder();
const byteLength = (text) => encoder.encode(text).length;

async function api(path, body) {
  const response = await fetch(path, {
    method: body ? "POST" : "GET",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const payload = await response.json();
  if (!response.ok) {
    if (response.status === 409) {
      await load();
      throw new Error("The file changed on disk, so the page reloaded. Nothing was lost — try again.");
    }
    throw new Error(payload.error || `Request failed (${response.status})`);
  }
  return payload;
}

function say(message, isError) {
  toast.textContent = message;
  toast.classList.toggle("error", Boolean(isError));
  toast.hidden = false;
  clearTimeout(say.timer);
  say.timer = setTimeout(() => (toast.hidden = true), isError ? 12000 : 5000);
}

async function run(work) {
  try {
    await work();
  } catch (err) {
    say(err.message, true);
  }
}

// Every text run the server rendered carries the byte offset it started at, so an
// offset is that number plus the bytes of rendered text preceding the caret.
function offsetAt(node, offset) {
  const element = node.nodeType === Node.TEXT_NODE ? node.parentElement : node;
  const span = element?.closest("[data-o]");
  if (!span) return null;
  const range = document.createRange();
  range.setStart(span, 0);
  range.setEnd(node, offset);
  return Number(span.dataset.o) + byteLength(range.toString());
}

// Endpoints can land outside any run — on a highlight boundary, or in whitespace
// between blocks. Fall back to the edge of the nearest run inside the selection;
// the server re-checks the quote and corrects small drifts.
function edgeOffset(range, wantEnd) {
  const spans = range.commonAncestorContainer.querySelectorAll?.("[data-o]") ?? [];
  const span = wantEnd ? spans[spans.length - 1] : spans[0];
  if (!span) return null;
  return Number(span.dataset.o) + (wantEnd ? byteLength(span.textContent) : 0);
}

// A selection only offers to become a comment. Nothing opens until the action is chosen,
// so the document stays readable and selectable.
function captureSelection() {
  hideSelectionMenu();

  const selection = window.getSelection();
  if (!selection || selection.isCollapsed || selection.rangeCount === 0) return;

  const range = selection.getRangeAt(0);
  if (!doc.contains(range.commonAncestorContainer)) return;

  const quote = selection.toString().trim();
  if (!quote) return;

  const start = offsetAt(range.startContainer, range.startOffset) ?? edgeOffset(range, false);
  const end = offsetAt(range.endContainer, range.endOffset) ?? edgeOffset(range, true);
  if (start === null || end === null || start >= end) {
    say("Could not locate that selection in the source. Try selecting whole words.", true);
    return;
  }

  state.pending = { start, end, quote };
  showSelectionMenu(range);
}

function showSelectionMenu(range) {
  const box = range.getBoundingClientRect();
  selectionMenu.hidden = false;

  const gap = 8;
  const width = selectionMenu.offsetWidth;
  const height = selectionMenu.offsetHeight;
  const left = clamp(box.left + box.width / 2 - width / 2, gap, window.innerWidth - width - gap);
  const above = box.top - height - gap;
  const top = above < gap ? box.bottom + gap : above;

  selectionMenu.style.left = `${left + window.scrollX}px`;
  selectionMenu.style.top = `${top + window.scrollY}px`;
}

function hideSelectionMenu() {
  selectionMenu.hidden = true;
}

function clamp(value, low, high) {
  return Math.min(Math.max(value, low), Math.max(low, high));
}

function openComposer(quote) {
  document.getElementById("composer-quote").textContent = quote;
  composer.hidden = false;
  document.getElementById("composer-body").focus();
}

function closeComposer() {
  composer.reset();
  composer.hidden = true;
  state.pending = null;
  hideSelectionMenu();
}

function element(tag, props = {}, ...children) {
  // dataset is read-only, so it cannot ride along with the rest.
  const { dataset, ...rest } = props;
  const node = Object.assign(document.createElement(tag), rest);
  Object.assign(node.dataset, dataset ?? {});
  node.append(...children.filter(Boolean));
  return node;
}

function select(name, value, options) {
  const node = element("select", { dataset: { field: name } });
  for (const option of options) {
    node.append(element("option", { value: option, textContent: option, selected: option === value }));
  }
  return node;
}

function threadCard(thread) {
  const resolved = thread.status === "resolved";
  const card = element("section", { className: `thread${resolved ? " resolved" : ""}` });
  card.dataset.id = thread.id;

  card.append(element("blockquote", { textContent: thread.quote }));

  for (const comment of thread.comments ?? []) {
    if (comment.deleted) continue;
    card.append(
      element(
        "p",
        { className: "comment" },
        element("span", { className: "byline", textContent: `${comment.author} · ${comment.ts}` }),
        comment.body,
      ),
    );
  }

  const meta = element("div", { className: "row" });
  for (const field of state.fields) {
    meta.append(
      element(
        "span",
        {},
        element("label", { textContent: field.label }),
        // Fields ride flat on the thread, the same way they are stored in the file.
        select(field.name, thread[field.name] || field.default, field.values),
      ),
    );
  }
  if (meta.children.length > 0) card.append(meta);

  card.append(element("textarea", { className: "reply", rows: 2, placeholder: "Reply…" }));
  card.append(
    element(
      "div",
      { className: "row" },
      element("button", { type: "button", dataset: { action: "reply" }, textContent: "Reply" }),
      element("button", {
        type: "button",
        className: "quiet",
        dataset: { action: "status" },
        textContent: resolved ? "Reopen" : "Resolve",
      }),
      element("button", { type: "button", className: "quiet", dataset: { action: "delete" }, textContent: "Delete" }),
    ),
  );
  return card;
}

function draw(payload) {
  state.rev = payload.rev;
  state.threads = payload.threads ?? [];
  state.fields = payload.fields ?? [];

  document.getElementById("submit-all").textContent = payload.updater
    ? `Submit all to ${payload.updater}`
    : "Submit all";
  document.getElementById("skill-name").textContent = payload.name || "skill-review";
  document.getElementById("skill-path").textContent = payload.path;
  document.title = payload.name ? `${payload.name} — skill-review` : "skill-review";

  doc.innerHTML = payload.html;
  for (const thread of state.threads) {
    if (thread.status === "resolved") {
      doc.querySelector(`.mc[data-id="${thread.id}"]`)?.classList.add("resolved");
    }
  }

  threadList.replaceChildren(...state.threads.map(threadCard));
  document.getElementById("empty").hidden = state.threads.length > 0;
  highlight(state.selected);
  hideSelectionMenu();
}

function highlight(id) {
  state.selected = id;
  for (const node of doc.querySelectorAll(".mc")) {
    node.classList.toggle("selected", node.dataset.id === id);
  }
  for (const card of threadList.children) {
    card.classList.toggle("selected", card.dataset.id === id);
  }
}

async function load() {
  draw(await api("/api/doc"));
}

doc.addEventListener("mouseup", () => setTimeout(captureSelection, 0));
doc.addEventListener("keyup", (event) => {
  if (event.shiftKey) captureSelection();
});

// The menu sits outside #doc, so its own mousedown must not count as clicking away.
document.addEventListener("mousedown", (event) => {
  if (!selectionMenu.contains(event.target)) hideSelectionMenu();
});

document.getElementById("selection-comment").addEventListener("click", () => {
  if (!state.pending) return;
  hideSelectionMenu();
  openComposer(state.pending.quote);
});

doc.addEventListener("click", (event) => {
  const mark = event.target.closest(".mc");
  if (!mark) return;
  highlight(mark.dataset.id);
  threadList.querySelector(`[data-id="${mark.dataset.id}"]`)?.scrollIntoView({ block: "nearest" });
});

composer.addEventListener("submit", (event) => {
  event.preventDefault();
  const body = document.getElementById("composer-body").value.trim();
  if (!body || !state.pending) return;

  run(async () => {
    draw(await api("/api/anchor", { rev: state.rev, ...state.pending, body }));
    closeComposer();
    window.getSelection()?.removeAllRanges();
    say("Comment saved to the file.");
  });
});

document.getElementById("composer-cancel").addEventListener("click", closeComposer);

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") hideSelectionMenu();
  if (event.key === "Escape" && !composer.hidden) closeComposer();
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter" && !composer.hidden) {
    composer.requestSubmit();
  }
});

threadList.addEventListener("click", (event) => {
  const button = event.target.closest("button[data-action]");
  if (!button) return;
  const card = button.closest(".thread");
  const id = card.dataset.id;
  const action = button.dataset.action;

  run(async () => {
    if (action === "delete") {
      draw(await api("/api/thread/delete", { rev: state.rev, id }));
      say("Thread removed.");
      return;
    }
    if (action === "status") {
      const status = card.classList.contains("resolved") ? "open" : "resolved";
      draw(await api("/api/thread", { rev: state.rev, id, status }));
      return;
    }
    const reply = card.querySelector(".reply");
    const body = reply.value.trim();
    if (!body) return;
    draw(await api("/api/thread", { rev: state.rev, id, body }));
    say("Reply saved to the file.");
  });
});

threadList.addEventListener("change", (event) => {
  const field = event.target.dataset?.field;
  if (!field) return;
  const id = event.target.closest(".thread").dataset.id;
  run(async () => {
    draw(await api("/api/thread", { rev: state.rev, id, fields: { [field]: event.target.value } }));
  });
});

// The prompt is the one thing worth keeping on screen, so this panel waits to be
// dismissed rather than timing out like every other message.
document.getElementById("submit-all").addEventListener("click", () => {
  run(async () => {
    const result = await api("/api/handoff", {});
    const count = result.payload.improvement_suggestions.length;
    const prompt = document.getElementById("handoff-prompt");

    if (count === 0) {
      document.getElementById("handoff-summary").textContent =
        "Nothing to hand off — every open thread has already been archived.";
      prompt.hidden = true;
      document.getElementById("handoff-copy").hidden = true;
      handoffPanel.hidden = false;
      return;
    }
    document.getElementById("handoff-copy").hidden = false;

    const noun = `${count} suggestion${count === 1 ? "" : "s"}`;
    document.getElementById("handoff-summary").textContent = result.changed
      ? `${noun} pending in ${result.file}`
      : `Nothing new — ${noun} still pending in ${result.file}`;
    prompt.textContent = result.prompt;
    prompt.hidden = false;
    handoffPanel.hidden = false;
    copyPrompt();
  });
});

async function copyPrompt() {
  const prompt = document.getElementById("handoff-prompt").textContent;
  try {
    await navigator.clipboard.writeText(prompt);
    say("Prompt copied to the clipboard.");
  } catch {
    say("Could not reach the clipboard — copy the prompt from the panel or the terminal.", true);
  }
}

document.getElementById("handoff-copy").addEventListener("click", () => copyPrompt());
document.getElementById("handoff-close").addEventListener("click", () => (handoffPanel.hidden = true));

run(load);
