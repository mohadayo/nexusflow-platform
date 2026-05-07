import request from "supertest";
import { app } from "./app";

describe("Dashboard Service", () => {
  describe("GET /health", () => {
    it("should return healthy status", async () => {
      const res = await request(app).get("/health");
      expect(res.status).toBe(200);
      expect(res.body.status).toBe("healthy");
      expect(res.body.service).toBe("dashboard");
      expect(res.body.timestamp).toBeDefined();
    });
  });

  describe("GET /metrics", () => {
    it("should return metrics", async () => {
      const res = await request(app).get("/metrics");
      expect(res.status).toBe(200);
      expect(res.body.uptime).toBeDefined();
      expect(res.body.requestCount).toBeGreaterThan(0);
      expect(res.body.services).toHaveLength(2);
    });
  });

  describe("GET /dashboard", () => {
    it("should return dashboard info", async () => {
      const res = await request(app).get("/dashboard");
      expect(res.status).toBe(200);
      expect(res.body.title).toBe("NexusFlow Dashboard");
      expect(res.body.version).toBe("1.0.0");
      expect(res.body.links).toBeDefined();
      expect(res.body.links.metrics).toBe("/metrics");
    });
  });

  describe("GET /services", () => {
    it("should return services status", async () => {
      const res = await request(app).get("/services");
      expect(res.status).toBe(200);
      expect(res.body.services).toBeDefined();
      expect(res.body.services).toHaveLength(2);
      res.body.services.forEach(
        (svc: { name: string; status: string; lastChecked: number }) => {
          expect(svc.name).toBeDefined();
          expect(svc.status).toBeDefined();
          expect(svc.lastChecked).toBeDefined();
        }
      );
    });
  });

  describe("GET /events/recent", () => {
    it("should handle unreachable event bus", async () => {
      const res = await request(app).get("/events/recent");
      expect(res.status).toBe(502);
      expect(res.body.error).toBeDefined();
    });
  });

  describe("GET /unknown", () => {
    it("should return 404 for unknown routes", async () => {
      const res = await request(app).get("/unknown");
      expect(res.status).toBe(404);
      expect(res.body.error).toBe("Not found");
    });
  });
});
