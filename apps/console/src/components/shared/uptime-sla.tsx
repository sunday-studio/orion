type UptimeDayBucket = {
  date?: string;
  total?: number;
  uptime_percent?: number;
};

export function UptimeSLA({ buckets, percent }: { buckets: UptimeDayBucket[]; percent?: number }) {
  return (
    <div className="uptime-sla">
      <p className="uptime-pct">{percent != null ? `${percent.toFixed(2)}%` : "—"} uptime</p>
      <div className="uptime-bars" role="img" aria-label="Daily uptime">
        {buckets.map((b) => {
          const p = b.uptime_percent ?? 0;
          let className = "bar";
          if (b.total === 0) className += " empty";
          else if (p >= 99) className += " up";
          else if (p >= 95) className += " degraded";
          else className += " down";
          return (
            <div key={b.date} className={className} title={`${b.date ?? ""}: ${p.toFixed(1)}%`} />
          );
        })}
      </div>
    </div>
  );
}
