import './style.css';
import './app.css';
import * as go from "../wailsjs/go/main/App.js";

const body = document.getElementById("msg-body");
const msg = document.getElementById("message-entry");
const sndBtn = document.getElementById("sendBtn");
const input = document.getElementById("textinput");

customElements.define("msg-entry", class extends HTMLElement {
  constructor() {
    super();

    this.contentElement = null;
    this.messageText = "";
    this.hasRendered = false;
  }

  connectedCallback() {
    if (this.hasRendered) {
      return;
    }

    const template = msg.content.cloneNode(true);
    this.appendChild(template);

    this.contentElement = this.querySelector(".msg-content");
    this.contentElement.innerText = this.messageText;

    this.hasRendered = true;
  }

  setText(text = "") {
    this.messageText = text;

    if (this.contentElement) {
      this.contentElement.innerText = text;
    }
  }
});

function addMessage(text = "") {
  const msgEntry = document.createElement("msg-entry");

  msgEntry.setText(text);

  body.appendChild(msgEntry);

  body.scrollTop = body.scrollHeight;
}

async function SendMessage() {
  const text = input.value.trim();

  if (text === "") {
    return;
  }

  addMessage(text);
  input.value = "";

  try {
    const response = await go.SendMsg(text);
    addMessage(response);
  } catch (error) {
    console.error(error);
    addMessage("Error contacting model.");
  }
}

sndBtn.addEventListener("click", SendMessage);
