// chat.js

const textarea = document.querySelector(".message-input");
const sendBtn = document.querySelector(".send-btn");

const room = sessionStorage.getItem("room");

if (!room) {
  alert("Room not found, redirecting...");
  window.location.href = "/";
}

const socket = new WebSocket(`ws://localhost:8080/ws/${room}`);

socket.onopen = () => {
  console.log("Connected to room:", room);
};

socket.onmessage = (event) => {
  console.log("Received:", event.data);
};

socket.onerror = (error) => {
  console.error("WebSocket error:", error);
};

socket.onclose = () => {
  console.log("Disconnected from server");
};

// Send message
sendBtn.addEventListener("click", () => {
  const msg = textarea.value.trim();
  if (!msg) return;

  if (socket.readyState === WebSocket.OPEN) {
    socket.send(msg);
    textarea.value = "";
  } else {
    alert("Not connected to WebSocket.");
  }
});

// Optional: send on Enter
textarea.addEventListener("keydown", (e) => {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    sendBtn.click();
  }
});
