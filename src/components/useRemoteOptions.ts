import { useEffect, useState } from 'react';
import type { ComboboxOption } from '@grafana/ui';

export type RemoteOptions = {
  options: Array<ComboboxOption<string>>;
  loading: boolean;
  /** Set when the names could not be listed, for example on a restricted user. */
  error?: string;
};

/** A load that finished, together with the loader it was made with. */
type Result = {
  load: () => Promise<string[]>;
  options: Array<ComboboxOption<string>>;
  error?: string;
};

function messageOf(error: unknown): string {
  if (typeof error === 'string') {
    return error;
  }
  if (error && typeof error === 'object') {
    // Grafana wraps a failed resource call into a FetchError, whose useful part
    // is the message the backend sent.
    const fetchError = error as { data?: { error?: string; message?: string }; statusText?: string; message?: string };
    return (
      fetchError.data?.error ??
      fetchError.data?.message ??
      fetchError.message ??
      fetchError.statusText ??
      'unknown error'
    );
  }
  return 'unknown error';
}

/**
 * Loads a list of names and turns it into combobox options, reloading whenever
 * `load` changes. Callers are expected to wrap it in `useCallback`, so that its
 * identity says when the result is out of date.
 *
 * Listing is a hint, not a requirement: a user without the rights to list the
 * databases of an instance still has to be able to type a name, so a failure is
 * reported rather than thrown.
 */
export function useRemoteOptions(load: () => Promise<string[]>): RemoteOptions {
  // The loader that produced the result is kept next to it, so that "still
  // loading" is derived rather than stored. Storing it would mean setting state
  // from inside the effect, which costs an extra render.
  const [result, setResult] = useState<Result | null>(null);

  useEffect(() => {
    let cancelled = false;

    load().then(
      (names) => {
        if (!cancelled) {
          setResult({ load, options: names.map((name) => ({ label: name, value: name })) });
        }
      },
      (error) => {
        if (!cancelled) {
          setResult({ load, options: [], error: messageOf(error) });
        }
      }
    );

    return () => {
      cancelled = true;
    };
  }, [load]);

  // Anything the current loader has not produced yet counts as loading.
  if (result === null || result.load !== load) {
    return { options: [], loading: true };
  }

  return { options: result.options, loading: false, error: result.error };
}
