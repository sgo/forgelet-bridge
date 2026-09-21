package steps

import (
	"context"
)

func fixturesRunning(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	for _, name := range forgeNames(captures[1]) {
		if _, err := w.forge(context.Background(), name); err != nil {
			return err
		}
		if err := w.startDashboard(name); err != nil {
			return err
		}
	}
	return nil
}

func configuredForgeAndOperator(_ context.Context, world any, captures []string) error {
	if err := configuredForges(context.Background(), world, []string{captures[0], captures[1]}); err != nil {
		return err
	}
	return configuredOperator(context.Background(), world, []string{captures[0], captures[2]})
}

func configuredOperator(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	w.operatorID = captures[1]
	return nil
}

func configuredForges(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	for _, name := range forgeNames(captures[1]) {
		if err := w.configureForge(name); err != nil {
			return err
		}
	}
	return w.configure(ctx)
}

func bridgeStarted(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return w.startBridge(ctx)
}

func bridgeRestarted(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	w.stopBridge()
	ctx, cancel := stepContext()
	defer cancel()
	return w.startBridge(ctx)
}

func bridgeCaughtUp(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return w.caughtUp(ctx)
}

func bridgeCreatedSpace(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	forgeName, roomName := captures[1], captures[2]
	ctx, cancel := stepContext()
	defer cancel()
	if err := w.startBridge(ctx); err != nil {
		return err
	}
	if _, err := w.waitForSpaceChild(ctx, forgeName, roomName); err != nil {
		return err
	}
	return nil
}
