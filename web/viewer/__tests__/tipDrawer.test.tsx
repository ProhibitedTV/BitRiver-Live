import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { viewerApiMocks } from "../test/test-utils";
import { TipDrawer } from "../components/TipDrawer";
import type { CryptoAddress } from "../lib/viewer-api";

const createTipMock = viewerApiMocks.createTip;

describe("TipDrawer", () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test("submits fractional tip amounts", async () => {
    const donationAddresses: CryptoAddress[] = [
      { currency: "btc", address: "bc1-test-address" }
    ];

    createTipMock.mockResolvedValue({} as any);

    const handleClose = jest.fn();
    const handleSuccess = jest.fn();

    const user = userEvent.setup();

    render(
      <TipDrawer
        open
        channelId="chan-123"
        channelTitle="Lo-fi Beats"
        donationAddresses={donationAddresses}
        onClose={handleClose}
        onSuccess={handleSuccess}
      />
    );

    const amountInput = screen.getByLabelText("Amount");
    const referenceInput = screen.getByLabelText("Wallet reference");

    await act(async () => {
      await user.type(amountInput, "0.0005");
      await user.type(referenceInput, "txn-123");
      await user.click(screen.getByRole("button", { name: /send tip/i }));
    });

    await waitFor(() => {
      expect(createTipMock).toHaveBeenCalledTimes(1);
      expect(createTipMock).toHaveBeenCalledWith(
        "chan-123",
        expect.objectContaining({ amount: 0.0005 })
      );
    });
  });
});
