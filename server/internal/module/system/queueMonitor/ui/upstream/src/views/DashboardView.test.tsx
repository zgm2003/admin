import React from "react";
import { render } from "@testing-library/react";
import { useQueueStatsRefresh } from "./DashboardView";

function Harness(props: { queueNames: string; refresh: () => void }) {
  useQueueStatsRefresh(props.queueNames, props.refresh);
  return null;
}

test("loads queue stats once after the initial queue list is known", () => {
  const refresh = jest.fn();
  const { rerender } = render(<Harness queueNames="" refresh={refresh} />);

  expect(refresh).not.toHaveBeenCalled();

  rerender(<Harness queueNames="default" refresh={refresh} />);
  expect(refresh).toHaveBeenCalledTimes(1);

  rerender(<Harness queueNames="default" refresh={refresh} />);
  expect(refresh).toHaveBeenCalledTimes(1);

  rerender(<Harness queueNames="critical,default" refresh={refresh} />);
  expect(refresh).toHaveBeenCalledTimes(2);

  rerender(<Harness queueNames="" refresh={refresh} />);
  expect(refresh).toHaveBeenCalledTimes(3);
});

test("loads queue stats immediately when queue data is already available", () => {
  const refresh = jest.fn();

  render(<Harness queueNames="default" refresh={refresh} />);

  expect(refresh).toHaveBeenCalledTimes(1);
});
