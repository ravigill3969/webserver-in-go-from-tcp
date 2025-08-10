const chat = [];
const textarea = document.querySelector(".message-input");
const sendBtn = document.querySelector(".send-btn");
const chatBox = document.querySelector("main.chat-messages");
const backBtn = document.querySelector(".back-btn");

// Get room from session storage
const room = sessionStorage.getItem("room");
if (!room) {
  alert("Room not found, redirecting...");
  window.location.href = "/";
}

// Setup WebSocket
const socket = new WebSocket(`ws://localhost:8080/ws/${room}`);

socket.onopen = () => {
  console.log("✅ Connected to room:", room);
  updateConnectionStatus(true);
};

socket.onmessage = (event) => {
  const msg = {
    myMessage: false,
    message: event.data,
    timestamp: new Date(),
  };
  chat.push(msg);
  renderChat();
};

socket.onerror = (err) => {
  console.error("❌ WebSocket error:", err);
  updateConnectionStatus(false);
};

socket.onclose = () => {
  console.log("🔌 Disconnected");
  updateConnectionStatus(false);
};

window.addEventListener("beforeunload", () => {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.close(1000, "User is leaving the page");
  }
});

// Send message
sendBtn.addEventListener("click", sendMessage);
textarea.addEventListener("keydown", (e) => {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    sendMessage();
  }
});

// Leave room
backBtn.addEventListener("click", () => {
  if (confirm("Leave chat room?")) {
    socket.close();
    window.location.href = "/";
  }
});

function sendMessage() {
  const msg = textarea.value.trim();
  if (!msg) return;

  if (socket.readyState === WebSocket.OPEN) {
    socket.send(msg);
    chat.push({
      myMessage: true,
      message: msg,
      timestamp: new Date(),
    });
    renderChat();
    textarea.value = "";
  } else {
    alert("Not connected.");
  }
}

function renderChat() {
  chatBox.innerHTML = ""; // Clear

  chat.forEach((msg) => {
    const el = document.createElement("div");
    el.className = `message ${msg.myMessage ? "self" : "other"}`;
    el.innerHTML = `
      ${
        !msg.myMessage
          ? `<div class="message-avatar"><i class="fas fa-user"></i></div>`
          : ""
      }
      <div class="message-content">
        <div class="message-header">
          ${!msg.myMessage ? `<span class="sender-name">Anonymous</span>` : ""}
          <span class="message-time">${formatTime(msg.timestamp)}</span>
        </div>
        <div class="message-text">${escapeHTML(msg.message)}</div>
      </div>
    `;
    chatBox.appendChild(el);
  });

  // Scroll to bottom
  chatBox.scrollTop = chatBox.scrollHeight;
}

function formatTime(date) {
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function updateConnectionStatus(isConnected) {
  const indicator = document.querySelector(".status-indicator");
  const status = document.querySelector(".room-status");

  indicator.style.background = isConnected ? "#10b981" : "#ef4444";
  status.innerHTML = `
    <span class="status-indicator"></span>
    ${isConnected ? "Connected" : "Disconnected"}
  `;
}

function escapeHTML(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}
