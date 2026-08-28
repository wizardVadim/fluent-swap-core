const elements = {
  url: document.querySelector("#socket-url"),
  connect: document.querySelector("#connect-button"),
  connectionStatus: document.querySelector("#connection-status"),
  searchForm: document.querySelector("#search-form"),
  nativeLanguage: document.querySelector("#native-language"),
  learningLanguage: document.querySelector("#learning-language"),
  find: document.querySelector("#find-button"),
  cancel: document.querySelector("#cancel-button"),
  searchStatus: document.querySelector("#search-status"),
  chat: document.querySelector("#chat-section"),
  matchID: document.querySelector("#match-id"),
  messages: document.querySelector("#messages"),
  messageForm: document.querySelector("#message-form"),
  messageInput: document.querySelector("#message-input"),
  log: document.querySelector("#event-log"),
};

let socket;
let activeMatchID;

function requestID(prefix) {
  const id = crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`;
  return `${prefix}-${id}`;
}

function socketIsOpen() {
  return socket?.readyState === WebSocket.OPEN;
}

function setConnectionStatus(text, state) {
  elements.connectionStatus.textContent = text;
  elements.connectionStatus.className = `status ${state}`;
}

function setSearching(searching) {
  elements.find.hidden = searching;
  elements.cancel.hidden = !searching;
  elements.find.disabled = searching || !socketIsOpen() || Boolean(activeMatchID);
  elements.nativeLanguage.disabled = searching || Boolean(activeMatchID);
  elements.learningLanguage.disabled = searching || Boolean(activeMatchID);
}

function log(direction, data) {
  const line = document.createElement("p");
  line.textContent = `${new Date().toLocaleTimeString()} ${direction} ${JSON.stringify(data)}`;
  elements.log.prepend(line);
}

function send(command) {
  if (!socketIsOpen()) {
    elements.searchStatus.textContent = "Нет соединения с сервером";
    return false;
  }
  socket.send(JSON.stringify(command));
  log("→", command);
  return true;
}

function connect() {
  if (socket?.readyState === WebSocket.CONNECTING || socketIsOpen()) {
    socket.close(1000, "Closed by user");
    return;
  }

  const url = elements.url.value.trim();
  if (!url.startsWith("ws://") && !url.startsWith("wss://")) {
    setConnectionStatus("URL должен начинаться с ws:// или wss://", "offline");
    return;
  }

  setConnectionStatus("Подключение…", "connecting");
  elements.connect.disabled = true;
  elements.url.disabled = true;
  socket = new WebSocket(url);

  socket.addEventListener("open", () => {
    setConnectionStatus("Подключено", "online");
    elements.connect.textContent = "Отключиться";
    elements.connect.disabled = false;
    elements.find.disabled = false;
    elements.searchStatus.textContent = "Готов к поиску";
    log("•", { event: "connection_opened" });
  });

  socket.addEventListener("message", ({ data }) => {
    try {
      const message = JSON.parse(data);
      log("←", message);
      handleServerMessage(message);
    } catch {
      log("!", { error: "invalid JSON from server", raw: data });
    }
  });

  socket.addEventListener("error", () => {
    setConnectionStatus("Ошибка соединения", "offline");
  });

  socket.addEventListener("close", ({ code }) => {
    setConnectionStatus("Не подключено", "offline");
    elements.connect.textContent = "Подключиться";
    elements.connect.disabled = false;
    elements.url.disabled = false;
    elements.find.disabled = true;
    setSearching(false);
    elements.searchStatus.textContent = "Соединение закрыто";
    activeMatchID = undefined;
    elements.chat.hidden = true;
    log("•", { event: "connection_closed", code });
  });
}

function handleServerMessage(message) {
  switch (message.type) {
    case "search_waiting":
      setSearching(true);
      elements.searchStatus.textContent = "Ищем подходящего собеседника…";
      break;
    case "search_cancelled":
      setSearching(false);
      elements.searchStatus.textContent = "Поиск отменён";
      break;
    case "match_found":
      activeMatchID = message.payload?.match_id;
      setSearching(false);
      elements.searchStatus.textContent = "Собеседник найден";
      elements.matchID.textContent = activeMatchID ?? "неизвестно";
      elements.chat.hidden = !activeMatchID;
      elements.messageInput.focus();
      break;
    case "receive_message":
      if (message.payload?.match_id === activeMatchID && typeof message.payload.text === "string") {
        appendMessage(message.payload.text, false);
      }
      break;
    case "error":
      if (!activeMatchID) setSearching(false);
      elements.searchStatus.textContent = `Ошибка: ${message.payload?.message ?? "неизвестная ошибка"}`;
      break;
    default:
      log("!", { warning: "unknown message type", type: message.type });
  }
}

function appendMessage(text, own) {
  elements.messages.querySelector(".empty")?.remove();
  const message = document.createElement("p");
  message.className = own ? "message own" : "message";
  message.textContent = text;
  elements.messages.append(message);
  elements.messages.scrollTop = elements.messages.scrollHeight;
}

elements.connect.addEventListener("click", connect);

elements.searchForm.addEventListener("submit", (event) => {
  event.preventDefault();
  if (elements.nativeLanguage.value === elements.learningLanguage.value) {
    elements.searchStatus.textContent = "Родной и изучаемый языки должны отличаться";
    return;
  }

  if (send({
    type: "find_partner",
    request_id: requestID("find"),
    payload: {
      native_language_code: elements.nativeLanguage.value,
      learning_language_code: elements.learningLanguage.value,
    },
  })) {
    elements.searchStatus.textContent = "Запрос отправлен…";
    elements.find.disabled = true;
  }
});

elements.cancel.addEventListener("click", () => {
  send({ type: "cancel_search", request_id: requestID("cancel") });
});

elements.messageForm.addEventListener("submit", (event) => {
  event.preventDefault();
  const text = elements.messageInput.value.trim();
  if (!text || !activeMatchID) return;

  if (new TextEncoder().encode(text).byteLength > 4096) {
    elements.searchStatus.textContent = "Сообщение превышает лимит 4096 UTF-8 байт";
    return;
  }

  if (send({
    type: "send_message",
    request_id: requestID("message"),
    payload: { text, match_id: activeMatchID },
  })) {
    appendMessage(text, true);
    elements.messageInput.value = "";
    elements.messageInput.focus();
  }
});

elements.messageInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    elements.messageForm.requestSubmit();
  }
});
