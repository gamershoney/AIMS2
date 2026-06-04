import './style.css';
import './app.css';
import * as go from "../wailsjs/go/main/App.js";

const body = document.getElementById("msg-body");
const msg = document.getElementById("message-entry");
const sndBtn = document.getElementById("sendBtn");
const input = document.getElementById("textinput");

const faction = {
  assistant: "assistant",
  user: "user"
}

customElements.define("msg-entry", class extends HTMLElement {
  constructor() {
    super();

    this.contentElement = null;
    this.messageText = "";
    this.hasRendered = false;
    this.faction = null
  }

  connectedCallback() {
    if (this.hasRendered) {
      return;
    }

    const template = msg.content.cloneNode(true);
    this.appendChild(template);

    this.contentElement = this.querySelector(".msg-content");
    this.contentElement.innerText = this.messageText;
    if (this.faction) {
      this.contentElement.classList.add(this.faction)
    }
    this.hasRendered = true;
  }

  setText(text = "", fact) {
    this.messageText = text;
    this.faction = fact
    console.log("Faction: ", fact)
    if (this.contentElement) {
      this.contentElement.innerText = text;
    }
  }
});

function addMessage(text = "", faction) {
  const msgEntry = document.createElement("msg-entry");

  msgEntry.setText(text, faction);

  body.appendChild(msgEntry);

  body.scrollTop = body.scrollHeight;
}

async function SendMessage() {
  const text = input.value.trim();

  if (text === "") {
    return;
  }

  addMessage(text, faction.user);
  input.value = "";

  try {
    const response = await go.SendMsg(text);
    addMessage(response, faction.assistant);
  } catch (error) {
    console.error(error);
    addMessage("Error contacting model.", faction.assistant);
  }
}

sndBtn.addEventListener("click", SendMessage);

addMessage("Good morning!", faction.assistant)
addMessage("Good morning!", faction.user)
