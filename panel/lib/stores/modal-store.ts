"use client";

import { create } from "zustand";

/**
 * Centralized modal stack so multiple call sites can open / close
 * dialogs without prop-drilling. Modals close in LIFO order on Escape.
 */
export interface ModalEntry {
  id: string;
  // Identifier of the modal component to render. The renderer maps this
  // to a concrete React component, keeping store state serializable.
  kind: string;
  props?: Record<string, unknown>;
}

interface ModalState {
  stack: ModalEntry[];
  open: (entry: Omit<ModalEntry, "id"> & { id?: string }) => string;
  close: (id?: string) => void;
  closeAll: () => void;
}

let nextId = 0;

export const useModalStore = create<ModalState>((set) => ({
  stack: [],
  open: (entry) => {
    const id = entry.id ?? `modal-${++nextId}`;
    set((state) => ({ stack: [...state.stack, { ...entry, id }] }));
    return id;
  },
  close: (id) =>
    set((state) => {
      if (!id) {
        return { stack: state.stack.slice(0, -1) };
      }
      return { stack: state.stack.filter((m) => m.id !== id) };
    }),
  closeAll: () => set({ stack: [] }),
}));
