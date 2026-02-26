import type { Message } from "../api/types.js";
import { isToolOnly } from "./content-parser.js";

export interface MessageItem {
  kind: "message";
  message: Message;
  ordinals: number[];
}

export interface ToolGroupItem {
  kind: "tool-group";
  messages: Message[];
  ordinals: number[];
  timestamp: string;
}

export interface SessionBoundaryItem {
  kind: "session-boundary";
  ordinals: number[];
}

export type DisplayItem =
  | MessageItem
  | ToolGroupItem
  | SessionBoundaryItem;

/**
 * Groups consecutive tool-only assistant messages into
 * compact display items. Non-tool messages pass through
 * as individual items.
 */
export function buildDisplayItems(
  messages: Message[],
): DisplayItem[] {
  const items: DisplayItem[] = [];
  let toolAcc: Message[] = [];
  let lastSessionId: string | undefined;

  function flushTools() {
    const [firstTool] = toolAcc;
    if (firstTool) {
      items.push({
        kind: "tool-group",
        messages: toolAcc,
        ordinals: toolAcc.map((m) => m.ordinal),
        timestamp: firstTool.timestamp,
      });
      toolAcc = [];
    }
  }

  for (const msg of messages) {
    // Insert session boundary when session_id changes
    if (
      lastSessionId !== undefined &&
      msg.session_id !== lastSessionId
    ) {
      flushTools();
      items.push({
        kind: "session-boundary",
        ordinals: [msg.ordinal],
      });
    }
    lastSessionId = msg.session_id;

    if (isToolOnly(msg)) {
      toolAcc.push(msg);
    } else {
      flushTools();
      items.push({
        kind: "message",
        message: msg,
        ordinals: [msg.ordinal],
      });
    }
  }

  flushTools();

  return items;
}
