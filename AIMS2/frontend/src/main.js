import './style.css';
import * as go from "../wailsjs/go/main/App.js";

const body = document.getElementById("msg-body");
const msg = document.getElementById("message-entry");
const sndBtn = document.getElementById("sendBtn");
const input = document.getElementById("textinput");
const removeBtn = document.getElementById("new-chat");
const emptyState = document.getElementById("empty-state");
const promptCards = document.getElementsByClassName("prompt-card");
const loader = document.getElementById("input-loader")
const appStatus = document.getElementById("app-status");
const statusLabel = document.getElementById("status-label");
const permissionModal = document.getElementById("permission-modal");
const permissionDescription = document.getElementById("permission-description");
const permissionTool = document.getElementById("permission-tool");
const permissionSkillRow = document.getElementById("permission-skill-row");
const permissionSkill = document.getElementById("permission-skill");
const permissionArgumentsWrap = document.getElementById("permission-arguments-wrap");
const permissionArguments = document.getElementById("permission-arguments");
const permissionError = document.getElementById("permission-error");
const approvePermissionBtn = document.getElementById("approve-permission");
const denyPermissionBtn = document.getElementById("deny-permission");
const faction = {
  assistant: "assistant",
  user: "user"
}

let workingMessage = null;
let isLoading = false;
let pendingPermission = null;

function formattedTime(date = new Date()) {
  return date.toLocaleTimeString([], {
    hour: "numeric",
    minute: "2-digit",
  });
}

function resizeInput() {
  input.style.height = "auto";
  input.style.height = `${input.scrollHeight}px`;
}

function setStatus(text, mode = "ready") {
  appStatus.dataset.mode = mode;
  statusLabel.textContent = text;
}

function setLoading(isLoading) {
  window.requestAnimationFrame(() => {
    resizeInput();
  });
  input.disabled = isLoading;
  sndBtn.disabled = isLoading;
  loader.classList.toggle("hidden", !isLoading);
  sndBtn.textContent = isLoading ? "Working" : "Send";
  input.setAttribute("aria-busy", String(isLoading));
  setStatus(isLoading ? "Working" : "Ready", isLoading ? "working" : "ready");
}

function displayToolName(name = "tool") {
  return name.replaceAll("_", " ").replace(/\b\w/g, character => character.toUpperCase());
}

function openPermissionModal(request) {
  pendingPermission = request;
  const toolName = displayToolName(request.tool);
  permissionTool.textContent = toolName;
  permissionDescription.textContent = request.description || `The workflow wants to run ${toolName}. This action may change stored data.`;

  const hasSkill = Boolean(request.skill_name);
  permissionSkillRow.classList.toggle("hidden", !hasSkill);
  permissionSkill.textContent = request.skill_name || "";

  const hasArguments = request.arguments && Object.keys(request.arguments).length > 0;
  permissionArgumentsWrap.classList.toggle("hidden", !hasArguments);
  permissionArguments.textContent = hasArguments ? JSON.stringify(request.arguments, null, 2) : "";
  permissionError.classList.add("hidden");
  permissionError.textContent = "";
  permissionModal.classList.remove("hidden");
  document.body.classList.add("permission-open");
  input.disabled = true;
  sndBtn.disabled = true;
  removeBtn.disabled = true;
  setStatus("Approval needed", "permission");
  window.requestAnimationFrame(() => approvePermissionBtn.focus());
}

function closePermissionModal() {
  permissionModal.classList.add("hidden");
  document.body.classList.remove("permission-open");
  input.disabled = false;
  sndBtn.disabled = false;
  removeBtn.disabled = false;
  pendingPermission = null;
}

async function checkForPendingPermission() {
  const request = await go.GetPendingPermission();
  if (request?.tool) {
    openPermissionModal(request);
    return true;
  }
  return false;
}

async function resolvePermission(approved) {
  if (!pendingPermission || isLoading) {
    return;
  }

  const request = pendingPermission;
  isLoading = true;
  approvePermissionBtn.disabled = true;
  denyPermissionBtn.disabled = true;
  permissionError.classList.add("hidden");
  setStatus(approved ? "Running approved action" : "Denying action", "working");

  try {
    const response = await go.ResolveToolPermission(approved);
    closePermissionModal();
    addMessage(`${approved ? "Allowed" : "Denied"}: ${displayToolName(request.tool)}`, faction.user, { variant: "permission-decision-entry" });
    addMessage(response?.trim() || (approved ? "The approved action completed." : "The action was not run."), faction.assistant);
    const hasAnotherRequest = await checkForPendingPermission();
    if (!hasAnotherRequest) {
      setStatus("Ready", "ready");
      input.focus();
    }
  } catch (error) {
    console.error(error);
    permissionError.textContent = "The decision could not be applied. Check the app logs and try again.";
    permissionError.classList.remove("hidden");
    setStatus("Permission error", "error");
  } finally {
    isLoading = false;
    approvePermissionBtn.disabled = false;
    denyPermissionBtn.disabled = false;
  }
}

customElements.define("msg-entry", class extends HTMLElement {
  constructor() {
    super();

    this.contentElement = null;
    this.labelElement = null;
    this.timeElement = null;
    this.messageText = "";
    this.hasRendered = false;
    this.faction = null
    this.timestamp = formattedTime();
  }

  connectedCallback() {
    if (this.hasRendered) {
      return;
    }

    const template = msg.content.cloneNode(true);
    this.appendChild(template);

    this.contentElement = this.querySelector(".msg-content");
    this.labelElement = this.querySelector(".message-label");
    this.timeElement = this.querySelector(".message-time");
    this.contentElement.textContent = this.messageText;
    if (this.faction) {
      this.contentElement.classList.add(this.faction)
      this.classList.add(`${this.faction}-entry`)
      this.labelElement.innerText = this.faction === faction.user ? "You" : "Assistant";
      this.timeElement.innerText = this.timestamp;
    }
    this.hasRendered = true;
  }

  setText(text = "", fact) {
    this.messageText = text;
    this.faction = fact
    if (this.contentElement) {
      this.contentElement.textContent = text;
    }
    if (this.labelElement) {
      this.labelElement.innerText = fact === faction.user ? "You" : "Assistant";
    }
    if (this.timeElement) {
      this.timeElement.innerText = this.timestamp;
    }
  }
});

function addMessage(text = "", faction, options = {}) {
  emptyState.classList.add("hidden");
  const msgEntry = document.createElement("msg-entry");

  msgEntry.setText(text, faction);
  if (options.variant) {
    msgEntry.classList.add(options.variant);
  }

  body.appendChild(msgEntry);

  body.scrollTop = body.scrollHeight;
  return msgEntry;
}

async function SendMessage() {
  const text = input.value.trim();
  if (text === "" || isLoading) {
    return;
  }

  isLoading = true;
  addMessage(text, faction.user);

  input.value = "";
  resizeInput();
  setLoading(true);
  workingMessage = addMessage("Working through the request…", faction.assistant, { variant: "working-entry" });
  let failed = false;
  try {
    const response = await go.SendMsg(text);
    workingMessage?.remove();
    addMessage(response?.trim() || "I did not receive a response from the model.", faction.assistant);
  } catch (error) {
    failed = true;
    console.error(error);
    workingMessage?.remove();
    addMessage("Error contacting the model. Check the app logs and try again.", faction.assistant, { variant: "error-entry" });
  } finally {
    workingMessage = null;
    isLoading = false;
    setLoading(false);
    if (failed) {
      setStatus("Error", "error");
    } else {
      try {
        await checkForPendingPermission();
      } catch (error) {
        console.error("Unable to check tool permission state", error);
        setStatus("Permission check failed", "error");
      }
    }
    if (!pendingPermission) {
      input.focus();
    }
  }
}

sndBtn.addEventListener("click", SendMessage);
input.addEventListener("input", resizeInput);
input.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    SendMessage();
  }
});

async function clearChat() {
  if (isLoading) {
    return;
  }
  setStatus("Clearing", "working");
  let resetFailed = false;
  try {
    await go.ClearChat();
  } catch (error) {
    resetFailed = true;
    console.error(error);
  }

  const msgs = body.children;
  [...msgs].forEach(e => {
    if (e.id !== "empty-state") {
      e.remove()
    }
  })
  emptyState.classList.remove("hidden");
  input.value = "";
  resizeInput();
  setLoading(false);
  workingMessage = null;
  if (resetFailed) {
    addMessage("The visible chat was cleared, but the backend chat reset failed. Restart the app if the next response seems out of context.", faction.assistant, { variant: "error-entry" });
    setStatus("Reset failed", "error");
  } else {
    setStatus("Ready", "ready");
  }
  input.focus();
}
removeBtn.addEventListener("click", clearChat);
approvePermissionBtn.addEventListener("click", () => resolvePermission(true));
denyPermissionBtn.addEventListener("click", () => resolvePermission(false));

[...promptCards].forEach(card => {
  card.addEventListener("click", () => {
    input.value = card.innerText;
    resizeInput();
    input.focus();
  });
});

resizeInput();
setStatus("Ready", "ready");
