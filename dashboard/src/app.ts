import express, { Request, Response, NextFunction } from "express";
import cors from "cors";

const app = express();

app.use(cors());
app.use(express.json());

const LOG_LEVEL = process.env.LOG_LEVEL || "info";

function log(level: string, message: string): void {
  if (level === "debug" && LOG_LEVEL !== "debug") return;
  const timestamp = new Date().toISOString();
  console.log(`${timestamp} [${level.toUpperCase()}] dashboard: ${message}`);
}

interface ServiceStatus {
  name: string;
  url: string;
  status: string;
  lastChecked: number;
}

interface DashboardMetrics {
  uptime: number;
  requestCount: number;
  services: ServiceStatus[];
}

let requestCount = 0;
const startTime = Date.now();

const EVENT_BUS_URL = process.env.EVENT_BUS_URL || "http://localhost:5001";
const GATEWAY_URL = process.env.GATEWAY_URL || "http://localhost:8080";

app.use((_req: Request, _res: Response, next: NextFunction) => {
  requestCount++;
  next();
});

app.get("/health", (_req: Request, res: Response) => {
  res.json({
    status: "healthy",
    service: "dashboard",
    timestamp: Date.now() / 1000,
  });
});

app.get("/metrics", (_req: Request, res: Response) => {
  const metrics: DashboardMetrics = {
    uptime: Math.floor((Date.now() - startTime) / 1000),
    requestCount,
    services: [
      {
        name: "event-bus",
        url: EVENT_BUS_URL,
        status: "configured",
        lastChecked: Date.now() / 1000,
      },
      {
        name: "api-gateway",
        url: GATEWAY_URL,
        status: "configured",
        lastChecked: Date.now() / 1000,
      },
    ],
  };
  log("info", `Metrics requested: ${requestCount} total requests`);
  res.json(metrics);
});

app.get("/dashboard", (_req: Request, res: Response) => {
  log("info", "Dashboard page requested");
  res.json({
    title: "NexusFlow Dashboard",
    version: "1.0.0",
    links: {
      metrics: "/metrics",
      health: "/health",
      services: "/services",
      events: "/events/recent",
    },
  });
});

app.get("/services", async (_req: Request, res: Response) => {
  const services = [
    { name: "event-bus", url: `${EVENT_BUS_URL}/health` },
    { name: "api-gateway", url: `${GATEWAY_URL}/health` },
  ];

  const results: ServiceStatus[] = [];

  for (const svc of services) {
    try {
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 5000);
      const resp = await fetch(svc.url, { signal: controller.signal });
      clearTimeout(timeout);
      results.push({
        name: svc.name,
        url: svc.url,
        status: resp.ok ? "healthy" : "unhealthy",
        lastChecked: Date.now() / 1000,
      });
    } catch {
      results.push({
        name: svc.name,
        url: svc.url,
        status: "unreachable",
        lastChecked: Date.now() / 1000,
      });
    }
  }

  log("info", `Services check: ${results.length} services checked`);
  res.json({ services: results });
});

app.get("/events/recent", async (_req: Request, res: Response) => {
  try {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 5000);
    const resp = await fetch(`${EVENT_BUS_URL}/events?limit=20`, {
      signal: controller.signal,
    });
    clearTimeout(timeout);
    if (!resp.ok) {
      log("warn", `Failed to fetch events: ${resp.status}`);
      res.status(502).json({ error: "Failed to fetch events from event bus" });
      return;
    }
    const data = await resp.json();
    res.json(data);
  } catch {
    log("error", "Event bus unreachable");
    res.status(502).json({ error: "Event bus is unreachable" });
  }
});

app.use((_req: Request, res: Response) => {
  res.status(404).json({ error: "Not found" });
});

export { app, requestCount, startTime };
