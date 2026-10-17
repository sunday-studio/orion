import { createServer } from "node:net";

/** Minimal SMTP server that accepts every message, for confirmation-email e2e flows. */
export const startSmtpSink = (port) =>
  new Promise((resolve, reject) => {
    const server = createServer((socket) => {
      let inData = false;
      let buffer = "";
      const reply = (line) => socket.write(`${line}\r\n`);
      reply("220 orion-smtp-sink ready");
      socket.on("data", (chunk) => {
        buffer += chunk.toString("utf8");
        if (inData) {
          if (buffer.includes("\r\n.\r\n")) {
            inData = false;
            buffer = "";
            reply("250 queued");
          }
          return;
        }
        let index;
        while ((index = buffer.indexOf("\r\n")) !== -1) {
          const line = buffer.slice(0, index).toUpperCase();
          buffer = buffer.slice(index + 2);
          if (line.startsWith("EHLO") || line.startsWith("HELO")) reply("250 orion-smtp-sink");
          else if (line.startsWith("DATA")) {
            inData = true;
            reply("354 end with <CRLF>.<CRLF>");
            if (buffer.includes("\r\n.\r\n")) {
              inData = false;
              buffer = "";
              reply("250 queued");
            }
            break;
          } else if (line.startsWith("QUIT")) {
            reply("221 bye");
            socket.end();
          } else reply("250 ok");
        }
      });
      socket.on("error", () => {});
    });
    server.once("error", reject);
    server.listen(port, "127.0.0.1", () => resolve(server));
  });
