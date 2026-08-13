const memory = new Map();

Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: {
    clear: () => memory.clear(),
    getItem: (key) => memory.has(String(key)) ? memory.get(String(key)) : null,
    key: (index) => Array.from(memory.keys())[index] ?? null,
    removeItem: (key) => memory.delete(String(key)),
    setItem: (key, value) => memory.set(String(key), String(value)),
    get length() { return memory.size; },
  },
});
