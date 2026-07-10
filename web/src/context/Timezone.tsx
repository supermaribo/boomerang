import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { api } from "../App";
import { guessBrowserTimezone } from "../lib/formatTime";

type Ctx = {
  timezone: string;
  setTimezone: (tz: string) => void;
  refreshTimezone: () => Promise<void>;
};

const TimezoneContext = createContext<Ctx>({
  timezone: "UTC",
  setTimezone: () => undefined,
  refreshTimezone: async () => undefined,
});

export function TimezoneProvider({ children }: { children: ReactNode }) {
  const [timezone, setTimezone] = useState("UTC");

  const refreshTimezone = useCallback(async () => {
    try {
      const me = await api<{ timezone?: string }>("/api/me");
      if (me.timezone) {
        setTimezone(me.timezone);
        return;
      }
    } catch {
      // not authed
    }
    setTimezone(guessBrowserTimezone());
  }, []);

  useEffect(() => {
    void refreshTimezone();
  }, [refreshTimezone]);

  return (
    <TimezoneContext.Provider value={{ timezone, setTimezone, refreshTimezone }}>
      {children}
    </TimezoneContext.Provider>
  );
}

export function useTimezone() {
  return useContext(TimezoneContext);
}
