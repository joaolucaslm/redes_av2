const ms3 = new Intl.NumberFormat("pt-BR", { minimumFractionDigits: 3, maximumFractionDigits: 3 });
const ms1 = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 });
const s2 = new Intl.NumberFormat("pt-BR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

export function timeLong(micros: number): string {
  return `${ms3.format(micros / 1000)} ms`;
}

export function timeShort(micros: number): string {
  if (Math.abs(micros) >= 10_000_000) {
    return `${s2.format(micros / 1_000_000)} s`;
  }
  return `${ms1.format(micros / 1000)} ms`;
}

export function percent(fraction: number): string {
  return `${Math.round(fraction * 100)}%`;
}

export function hostClass(host: string | undefined): string {
  return host === "B" ? "host-b" : "host-a";
}

export function hostColor(host: string | undefined): string {
  return host === "B" ? "var(--phos-b)" : "var(--phos-a)";
}

export function reasonLabel(reason: string | undefined): string {
  switch (reason) {
    case "timeout":
      return "timeout";
    case "fast":
      return "rápida";
    case "duplicate":
      return "SYN repetido";
    case "reenvio":
      return "reenvio";
    default:
      return reason ?? "";
  }
}

export function protocolLabel(id: string): string {
  return id.replaceAll("-", " ").toUpperCase();
}
