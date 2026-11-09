Feature: Label app-originated forge requests

  # A completed-card report created by the Forgelet app reaches the chat room
  # under the bridge's Matrix account. Matrix cannot make that event look as
  # though the operator sent it. The bridge therefore reads the dashboard's
  # safe source metadata and gives only app reports an explicit relay label,
  # while the posted event remains the thread root for the Lieutenant's answer.
  # The source is metadata, not a guess made from report text. Requests without
  # it, and messages the operator writes in Matrix, keep their existing
  # pairing and threading behavior.

  Background:
    Given the fixture forge root forge-a has its dashboard running
    And the bridge is configured with the forge root forge-a and the operator @operator:example.org
    And the bridge is started

  Scenario Outline: App-originated reports are explicitly relayed and answered in their own thread
    Given the forge's dashboard already holds the app report "<report>" from source app.completed-card-report
    Then the operator decrypts an app-relayed report reading "<expected_report>"
    And the bridge sent the app-relayed report "<expected_report>" encrypted
    And the forge holds exactly one chat request reading "<expected_report>"
    When the lieutenant answers the chat request "<report>" with "<answer>"
    Then the operator decrypts the thread reply "<expected_answer>" to the app-relayed report "<expected_report>"

    Examples:
      | report                                                   | expected_report                                          | answer                              | expected_answer                     |
      | Completed card report: bridge release is ready            | Completed card report: bridge release is ready            | All reported cards were verified.   | All reported cards were verified.   |

  Scenario Outline: App-originated reports remain distinct with prior bridge state
    Given the forge's dashboard already holds the chat request "<prior_request>"
    And the lieutenant answers the chat request "<prior_request>" with "<prior_answer>"
    And the bridge has caught up with the forge
    And the forge's dashboard already holds the app report "<report>" from source app.completed-card-report
    And the operator decrypts an app-relayed report reading "<expected_report>"
    When the lieutenant answers the chat request "<report>" with "<answer>"
    Then the operator decrypts the thread reply "<expected_answer>" to the app-relayed report "<expected_report>"
    And the operator decrypts the thread reply "<expected_prior_answer>" to the chat message "<expected_prior_request>"

    Examples:
      | prior_request          | expected_prior_request | prior_answer              | expected_prior_answer     | report                                      | expected_report                             | answer                       | expected_answer               |
      | is the forge idle?     | is the forge idle?     | Yes, every lane is quiet. | Yes, every lane is quiet. | Completed card report: cleanup is finished. | Completed card report: cleanup is finished. | The completed card is gone. | The completed card is gone. |

  Scenario Outline: A restart does not replay an app-relayed report
    Given the forge's dashboard already holds the app report "<report>" from source app.completed-card-report
    And the operator decrypts an app-relayed report reading "<expected_report>"
    And the lieutenant answers the chat request "<report>" with "<answer>"
    And the bridge has caught up with the forge
    When the bridge is stopped and started again
    Then chat room Chat holds exactly one app-relayed report reading "<expected_report>"
    And chat room Chat holds exactly one thread reply reading "<expected_answer>"

    Examples:
      | report                              | expected_report                     | answer                         | expected_answer                |
      | Completed card report: parser work. | Completed card report: parser work. | Parser work was verified.       | Parser work was verified.       |

  Scenario Outline: Requests without source metadata retain ordinary presentation
    Given the forge's dashboard already holds the chat request "<request>"
    When the lieutenant answers the chat request "<request>" with "<answer>"
    Then the operator decrypts the chat message "<expected_request>"
    And the operator decrypts the thread reply "<expected_answer>" to the chat message "<expected_request>"
    And the forge holds exactly one chat request reading "<expected_request>"

    Examples:
      | request                    | expected_request           | answer                   | expected_answer          |
      | is the build green?        | is the build green?        | Yes, the build is green. | Yes, the build is green. |

  Scenario Outline: Operator Matrix messages retain their own pairing and thread
    When the operator sends the message "<request>" into chat room Chat
    And the lieutenant answers the chat request "<request>" with "<answer>"
    Then the forge holds exactly one chat request reading "<expected_request>"
    And the operator decrypts the thread reply "<expected_answer>" to the chat message "<expected_request>"

    Examples:
      | request                     | expected_request            | answer                        | expected_answer               |
      | is the deployment healthy?  | is the deployment healthy?  | Yes, every forge is reached. | Yes, every forge is reached. |
