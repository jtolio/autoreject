# autoreject

A Google App Engine app.

Google Calendar has a nice "Out of Office" event feature, where if you
schedule an "Out of Office" event, event invites during that time will be
automatically declined. Unfortunately, you can't make "Out of Office" events
be recurring.

This application watches your calendar and automatically rejects events
you are invited to if they conflict with special calendar events you create.

If you make sure the name of an event on your calendar includes the
configured "Autoreject identifier" in the event name, then other events scheduled
during that time will be rejected with the configured "Autoreject reply."
