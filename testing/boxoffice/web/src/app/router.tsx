// The web's app layer: routes to pages, one router for the site.
import {
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  Outlet,
} from "@tanstack/react-router";

import { EventPage } from "../pages/event";
import { EventsPage } from "../pages/events";
import { OrderPage } from "../pages/order";
import { OrganizerPage } from "../pages/organizer";
import { OrganizerEventPage } from "../pages/organizer-event";
import { PageShell } from "../shared/ui";

const rootRoute = createRootRoute({
  component: () => (
    <PageShell title="boxoffice">
      <nav className="nav">
        <Link to="/" activeOptions={{ exact: true }} activeProps={{ className: "active" }}>
          On sale
        </Link>
        <Link to="/organizer" activeProps={{ className: "active" }}>
          Organizer
        </Link>
      </nav>
      <Outlet />
    </PageShell>
  ),
});

const eventsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: EventsPage,
});

const eventRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/events/$eventId",
  component: EventPage,
});

const orderRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/orders/$orderId",
  component: OrderPage,
});

const organizerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/organizer",
  component: OrganizerPage,
});

const organizerEventRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/organizer/events/$eventId",
  component: OrganizerEventPage,
});

export const router = createRouter({
  routeTree: rootRoute.addChildren([
    eventsRoute,
    eventRoute,
    orderRoute,
    organizerRoute,
    organizerEventRoute,
  ]),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
