import { renderHook, waitFor } from '@testing-library/react';
import { useRemoteOptions } from './useRemoteOptions';

describe('useRemoteOptions', () => {
  it('turns the loaded names into options', async () => {
    const load = jest.fn().mockResolvedValue(['alpha', 'beta']);

    const { result } = renderHook(() => useRemoteOptions(load));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.options).toEqual([
      { label: 'alpha', value: 'alpha' },
      { label: 'beta', value: 'beta' },
    ]);
    expect(result.current.error).toBeUndefined();
  });

  it('reports loading while the names are being fetched', async () => {
    const load = jest.fn().mockReturnValue(new Promise(() => {}));

    const { result } = renderHook(() => useRemoteOptions(load));

    await waitFor(() => expect(result.current.loading).toBe(true));
  });

  // Listing is a hint, not a requirement, so a failure must not throw.
  it.each([
    ['a backend message', { data: { error: 'not authorized' } }, 'not authorized'],
    ['an error instance', new Error('boom'), 'boom'],
    ['a plain string', 'nope', 'nope'],
    ['something unexpected', 42, 'unknown error'],
  ])('reports %s as an error', async (_name, rejection, expected) => {
    const load = jest.fn().mockRejectedValue(rejection);

    const { result } = renderHook(() => useRemoteOptions(load));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe(expected);
    expect(result.current.options).toEqual([]);
  });

  // The loader identity is what says the result is out of date, so callers
  // memoize it with the values it depends on.
  it('reloads when the loader changes', async () => {
    const first = jest.fn().mockResolvedValue(['alpha']);
    const second = jest.fn().mockResolvedValue(['beta']);

    const { result, rerender } = renderHook(({ load }) => useRemoteOptions(load), {
      initialProps: { load: first },
    });

    await waitFor(() => expect(result.current.options).toEqual([{ label: 'alpha', value: 'alpha' }]));

    rerender({ load: second });
    await waitFor(() => expect(result.current.options).toEqual([{ label: 'beta', value: 'beta' }]));

    // The same loader does not trigger another call.
    rerender({ load: second });
    expect(second).toHaveBeenCalledTimes(1);
  });

  // A slow first request must not overwrite the result of a newer one.
  it('ignores the result of a superseded load', async () => {
    let resolveFirst: (names: string[]) => void = () => {};
    const first = jest.fn().mockImplementation(() => new Promise<string[]>((resolve) => (resolveFirst = resolve)));
    const second = jest.fn().mockResolvedValue(['second']);

    const { result, rerender } = renderHook(({ load }) => useRemoteOptions(load), {
      initialProps: { load: first },
    });

    rerender({ load: second });
    await waitFor(() => expect(result.current.options).toEqual([{ label: 'second', value: 'second' }]));

    resolveFirst(['first']);
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(result.current.options).toEqual([{ label: 'second', value: 'second' }]);
  });
});
