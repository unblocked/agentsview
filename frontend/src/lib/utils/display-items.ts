import type { Message } from "../api/types.js";
import { isToolOnly } from "./content-parser.js";

export type DisplayItem =
  | { kind: "message"; message: Message; ordinals: number[] }
  | {
      kind: "tool-group";
      messages: Message[];
      ordinals: number[];
      timestamp: string;
    };

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

  function flushTools() {
    if (toolAcc.length === 0) return;
    items.push({
      kind: "tool-group",
      messages: toolAcc,
      ordinals: toolAcc.map((m) => m.ordinal),
      timestamp: toolAcc[0]!.timestamp,
    });
    toolAcc = [];
  }

  for (const msg of messages) {
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
