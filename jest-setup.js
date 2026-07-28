// Jest setup provided by Grafana scaffolding
import './.config/jest-setup';

// The Combobox used by the query editor sizes its dropdown by measuring text on
// a canvas, and its scroll container observes the elements it holds. jsdom
// provides none of that, and the scaffolded setup stubs getContext into
// returning undefined, so they are filled in here.
HTMLCanvasElement.prototype.getContext = () => ({
  measureText: (text) => ({ width: String(text).length * 8 }),
});

class NoopObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
}

global.ResizeObserver = global.ResizeObserver || NoopObserver;
global.IntersectionObserver = global.IntersectionObserver || NoopObserver;

// The dropdown list is virtualized, so it only renders the rows that fit in the
// visible area. jsdom reports every element as zero sized, which would render
// no row at all, so elements are given a size.
Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 600 });
Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 600 });
Element.prototype.getBoundingClientRect = function getBoundingClientRect() {
  return { width: 600, height: 600, top: 0, left: 0, bottom: 600, right: 600, x: 0, y: 0, toJSON: () => ({}) };
};
