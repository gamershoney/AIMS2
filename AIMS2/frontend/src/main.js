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
const faction = {
  assistant: "assistant",
  user: "user"
}

let workingMessage = null;

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

function setLoading(isLoading) {
  input.disabled = isLoading;
  sndBtn.disabled = isLoading;
  loader.classList.toggle("hidden", !isLoading);
  input.setAttribute("aria-busy", String(isLoading));
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
    this.contentElement.innerText = this.messageText;
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
      this.contentElement.innerText = text;
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
  if (text === "" || sndBtn.disabled) {
    return;
  }

  addMessage(text, faction.user);

  input.value = "";
  resizeInput();
  setLoading(true);
  workingMessage = addMessage("Assistant is working…", faction.assistant, { variant: "working-entry" });
  try {
    const response = await go.SendMsg(text);
    workingMessage?.remove();
    addMessage(response, faction.assistant);
  } catch (error) {
    console.error(error);
    workingMessage?.remove();
    addMessage("Error contacting model.", faction.assistant);
  } finally {
    workingMessage = null;
    setLoading(false);
    input.focus();
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

function clearChat() {
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
  input.focus();
}
removeBtn.addEventListener("click", clearChat);

[...promptCards].forEach(card => {
  card.addEventListener("click", () => {
    input.value = card.innerText;
    resizeInput();
    input.focus();
  });
});

resizeInput();
