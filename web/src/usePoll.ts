import { useEffect, useRef, useState } from "react";

type State<T> = { data: T | null; error: string | null; loading: boolean };

// หยุด poll เมื่อ component ถูกถอดออก ไม่งั้นแท็บที่เปิดค้างจะยิงต่อไปเรื่อย ๆ
export function usePoll<T>(load: () => Promise<T>, everyMs: number): State<T> & { reload: () => void } {
  const [state, setState] = useState<State<T>>({ data: null, error: null, loading: true });
  const loadRef = useRef(load);
  loadRef.current = load;
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let alive = true;

    const tick = async () => {
      try {
        const data = await loadRef.current();
        if (alive) setState({ data, error: null, loading: false });
      } catch (err) {
        if (alive) {
          setState((s) => ({ data: s.data, error: (err as Error).message, loading: false }));
        }
      }
    };

    void tick();
    if (everyMs <= 0) return () => { alive = false; };

    const id = setInterval(tick, everyMs);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [everyMs, nonce]);

  return { ...state, reload: () => setNonce((n) => n + 1) };
}
